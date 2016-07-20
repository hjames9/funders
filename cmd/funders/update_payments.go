package main

import (
	"bitbucket.org/savewithus/funders"
	"encoding/json"
	"fmt"
	"github.com/martini-contrib/binding"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	UPDATE_PAYMENT_QUERY = "UPDATE funders.payments SET updated_at = $1, payment_processor_responses = payment_processor_responses || $2, payment_processor_used = $3, status = $4 WHERE id = $5"
)

type UpdatePayment struct {
	Id              string `form:"id" binding:"required"`
	AccountType     string `form:"accountType" binding:"required"`
	Status          string
	PaypalPayerId   string `form:"paypalPayerId"`
	PaypalPaymentId string `form:"paypalPaymentId"`
	PaypalToken     string `form:"paypalToken"`
	payment         *Payment
}

func (updatePayment *UpdatePayment) MarshalJSON() ([]byte, error) {
	type MyUpdatePayment UpdatePayment
	return json.Marshal(&struct {
		Id            string    `json:"id"`
		CampaignId    int64     `json:"campaignId"`
		Campaign      *Campaign `json:"campaign"`
		PerkId        int64     `json:"perkId"`
		Perk          *Perk     `json:"perk"`
		Status        string    `json:"status"`
		FailureReason string    `json:"failureReason,omitempty"`
	}{
		Id:            updatePayment.Id,
		CampaignId:    updatePayment.payment.CampaignId,
		Campaign:      updatePayment.payment.Campaign,
		PerkId:        updatePayment.payment.PerkId,
		Perk:          updatePayment.payment.Perk,
		Status:        updatePayment.payment.Status,
		FailureReason: updatePayment.payment.FailureReason,
	})
}

func (updatePayment *UpdatePayment) Validate(errors binding.Errors, req *http.Request) binding.Errors {
	errors = validateSizeLimit(updatePayment.Id, "id", stringSizeLimit, errors)
	errors = validateSizeLimit(updatePayment.AccountType, "accountType", stringSizeLimit, errors)
	errors = validateSizeLimit(updatePayment.PaypalPayerId, "paypalPayerId", stringSizeLimit, errors)
	errors = validateSizeLimit(updatePayment.PaypalPaymentId, "paypalPaymentId", stringSizeLimit, errors)
	errors = validateSizeLimit(updatePayment.PaypalToken, "paypalToken", stringSizeLimit, errors)

	if len(errors) == 0 {
		if !accountTypes[updatePayment.AccountType] {
			message := fmt.Sprintf("Invalid account type \"%s\" specified", updatePayment.AccountType)
			errors = addError(errors, []string{"accountType"}, binding.TypeError, message)
		}

		if updatePayment.AccountType == "paypal" && (len(updatePayment.PaypalPayerId) == 0 || len(updatePayment.PaypalPaymentId) == 0 || len(updatePayment.PaypalToken) == 0) {
			errors = addError(errors, []string{"accountType", "paypalPayerId", "paypalPaymentId", "paypalToken"}, binding.RequiredError, "PaypalPayerId, PaypalPaymentId, and PaypalToken required to update a paypal payment")
		}

		var err error
		updatePayment.payment, err = getPayment(updatePayment.Id)
		if nil != err {
			message := fmt.Sprintf("Payment id %s not found", updatePayment.Id)
			errors = addError(errors, []string{"id"}, binding.TypeError, message)
		} else {
			log.Printf("Found payment %s", updatePayment.Id)
			if !strings.EqualFold(updatePayment.payment.AccountType, updatePayment.AccountType) {
				message := fmt.Sprintf("Received account type %s does not match existing payment account type %s", updatePayment.AccountType, updatePayment.payment.AccountType)
				errors = addError(errors, []string{"accountType"}, binding.TypeError, message)
			} else {
				//Get campaign and perk
				perk, exists := perks.GetPerk(updatePayment.payment.PerkId)
				if exists {
					updatePayment.payment.Perk = perk
				} else {
					log.Printf("Could not find perk %d for campaign %d for payment %s", updatePayment.payment.PerkId, updatePayment.payment.CampaignId, updatePayment.Id)
				}

				campaign, exists := campaigns.GetCampaignById(updatePayment.payment.CampaignId)
				if exists {
					updatePayment.payment.Campaign = campaign
				} else {
					log.Printf("Could not find campaign %d for payment %s", updatePayment.payment.CampaignId, updatePayment.Id)
				}
			}

			updatePayment.Status = updatePayment.payment.Status

			if updatePayment.payment.Status != "pending" {
				message := fmt.Sprintf("Only pending payments can be updated. Status: \"%s\" specified", updatePayment.payment.Status)
				errors = addError(errors, []string{"status"}, binding.TypeError, message)
			}
		}

		if botDetection.IsBot(req) {
			message := "Go away spambot! We've alerted the authorities"
			errors = addError(errors, []string{"spambot"}, common.BOT_ERROR, message)
		}
	}

	return errors
}

//Asynchronous payments
var asyncUpdatePaymentRequest bool

//Background payment threads
var updatePaymentBatchProcessor *common.BatchProcessor

func processUpdatePayment(updatePayment *UpdatePayment) (error, int) {
	var err error
	var retCode int

	switch updatePayment.AccountType {
	case "paypal":
		err = executePaypalPayment(updatePayment, nil)
	case "credit_card":
		fallthrough
	case "bitcoin":
		fallthrough
	default:
		err = common.RequestError{fmt.Sprintf("Unsupported payment update account type %s", updatePayment.AccountType), common.ServiceNotImplementedError}
	}

	if nil == err {
		//Success: StatusOK (successful transaction from paypal and database)
		retCode = http.StatusOK
	} else if err, found := err.(common.RequestError); found {
		switch common.RequestError(err).Type {
		case common.BadRequestError:
			//Error: StatusBadRequest (paypal couldn't process bad payment data)
			retCode = http.StatusBadRequest
		case common.NotFoundError:
			//Error: StatusNotFound(couldn't process previous payment)
			retCode = http.StatusNotFound
		case common.ServiceUnavailableError:
			//Error: StatusServiceUnavailable (paypal or database is down)
			retCode = http.StatusServiceUnavailable
		case common.ServiceNotImplementedError:
			//Error: StatusNotImplemented (paypal or database is down)
			retCode = http.StatusNotImplemented
		case common.ServerError:
			fallthrough
		default:
			//Error: StatusInternalServerError (paypal or database had some processing error)
			retCode = http.StatusInternalServerError
		}
	} else {
		retCode = http.StatusInternalServerError
	}

	return err, retCode
}

func processBatchUpdatePayment(updatePaymentBatch []interface{}, waitGroup *sync.WaitGroup) {
	log.Printf("Starting batch processing of %d updatePayments", len(updatePaymentBatch))
	defer waitGroup.Done()

	for _, updatePaymentInterface := range updatePaymentBatch {
		updatePayment := updatePaymentInterface.(*UpdatePayment)
		switch updatePayment.AccountType {
		case "paypal":
			waitGroup.Add(1)
			go executePaypalPayment(updatePayment, waitGroup)
		case "credit_card":
			fallthrough
		case "bitcoin":
			fallthrough
		default:
			log.Printf("Unsupported payment update account type %s", updatePayment.AccountType)
		}
	}
}

func updatePaymentInDb(payment *Payment) (*Payment, error) {
	_, err := db.Exec(UPDATE_PAYMENT_QUERY, time.Now(), payment.PaymentProcessorResponses, payment.PaymentProcessorUsed, payment.GetStatus(), payment.Id)
	return payment, err
}

func updatePaymentHandler(res http.ResponseWriter, req *http.Request, updatePayment UpdatePayment) (int, string) {
	req.Close = true
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	var response common.Response

	log.Printf("Received new payment update: %#v", updatePayment)

	if asyncUpdatePaymentRequest && updatePaymentBatchProcessor.Running {
		updatePaymentBatchProcessor.AddEvent(&updatePayment)
		responseStr := "Successfully scheduled payment update"
		response = common.Response{Code: http.StatusAccepted, Message: responseStr, Id: updatePayment.Id}
		log.Print(responseStr)
	} else if !asyncUpdatePaymentRequest {
		err, retCode := processUpdatePayment(&updatePayment)
		if nil != err {
			response = common.Response{Code: retCode, Message: err.Error(), Id: updatePayment.Id}
			log.Print(err)
		} else {
			jsonStr, _ := json.Marshal(&updatePayment)
			return retCode, string(jsonStr)
		}
	} else if asyncUpdatePaymentRequest && (nil == updatePaymentBatchProcessor || !updatePaymentBatchProcessor.Running) {
		responseStr := "Could not add payment update due to server maintenance"
		response = common.Response{Code: http.StatusServiceUnavailable, Message: responseStr, Id: updatePayment.Id}
		log.Print(responseStr)
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}
