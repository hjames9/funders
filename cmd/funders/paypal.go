package main

import (
	"encoding/json"
	"fmt"
	"github.com/hjames9/funders"
	"github.com/logpacker/PayPal-Go-SDK"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

const (
	PAYPAL_PROCESSOR = "paypal"
)

var paypalClient *paypalsdk.Client

func makePaypalPayment(payment *Payment, waitGroup *sync.WaitGroup) error {
	if nil != waitGroup {
		defer waitGroup.Done()
	}

	if payment.AccountType != "paypal" {
		return common.RequestError{"Only paypal payments are currently supported", common.ServiceNotImplementedError}
	}

	payment.PaymentProcessorUsed = PAYPAL_PROCESSOR

	campaign, campaignExists := campaigns.GetCampaignById(payment.CampaignId)
	if !campaignExists {
		return common.RequestError{fmt.Sprintf("Campaign not found %d", payment.CampaignId), common.NotFoundError}
	}

	perk, perkExists := perks.GetPerk(payment.PerkId)
	if !perkExists {
		return common.RequestError{fmt.Sprintf("Perk not found %d", payment.PerkId), common.NotFoundError}
	}

	amount := paypalsdk.Amount{
		Total:    strconv.FormatFloat(payment.Amount, 'f', -1, 64),
		Currency: payment.Currency,
	}

	hasParameters := func(uri string) (bool, string) {
		if strings.Contains(uri, "?") {
			return true, "&"
		} else {
			return false, "?"
		}
	}

	_, redirectURIDelim := hasParameters(payment.PaypalRedirectUrl)
	_, cancelURIDelim := hasParameters(payment.PaypalCancelUrl)

	redirectURI := fmt.Sprintf("%s%spaymentId=%s", payment.PaypalRedirectUrl, redirectURIDelim, payment.Id)
	cancelURI := fmt.Sprintf("%s%spaymentId=%s", payment.PaypalCancelUrl, cancelURIDelim, payment.Id)
	description := fmt.Sprintf("Perk(%s) for Campaign(%s). Payment id(%s)", perk.Name, campaign.Name, payment.Id)
	paymentResult, err := paypalClient.CreateDirectPaypalPayment(amount, redirectURI, cancelURI, description)

	if nil == err {
		jsonStr, jsonErr := json.Marshal(paymentResult)
		if nil == jsonErr {
			payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(string(jsonStr), "\"", "\\\"", -1))
		} else {
			log.Print(jsonErr)
			log.Printf("Unable to marshal payment response (%#v) from paypal", paymentResult)
		}

		//Set approval url
		payment.PaypalApprovalUrl = paymentResult.Links[0].Href
	} else {
		log.Printf("%#v", err)
		log.Print("Failed processing payment with processor")

		jsonStr, jsonErr := json.Marshal(err)
		if nil != jsonErr {
			log.Print(jsonErr)
			log.Print("Unable to marshal paypal error")
		} else {
			payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(string(jsonStr), "\"", "\\\"", -1))
		}

		payment.UpdateStatus("failure")

		if _, urlErr := url.Parse(redirectURI); nil != urlErr {
			err = common.RequestError{fmt.Sprintf("Redirect URI is invalid: %s", redirectURI), common.BadRequestError}
		} else if _, urlErr := url.Parse(cancelURI); nil != urlErr {
			err = common.RequestError{fmt.Sprintf("Cancel URI is invalid: %s", cancelURI), common.BadRequestError}
		} else if nil != paymentResult && len(paymentResult.ID) == 0 {
			err = common.RequestError{err.Error(), common.ServerError}
		} else {
			//Seems like only server errors should occur here
			err = common.RequestError{err.Error(), common.ServerError}
		}
	}

	paymentsCache.AddOrReplacePayment(payment)
	if nil != waitGroup {
		_, dbErr := updatePaymentInDb(payment)
		if nil != dbErr {
			log.Print(dbErr)
			log.Printf("Error updating payment %s with information from processor", payment.Id)
			log.Print("%#v", payment)
		} else {
			log.Printf("Successfully updated payment %s in database", payment.Id)
		}
	}

	return err
}

func executePaypalPayment(updatePayment *UpdatePayment, waitGroup *sync.WaitGroup) error {
	if nil != waitGroup {
		defer waitGroup.Done()
	}

	payment := updatePayment.payment

	campaign, campaignExists := campaigns.GetCampaignById(payment.CampaignId)
	if !campaignExists {
		return common.RequestError{fmt.Sprintf("Campaign not found %d", payment.CampaignId), common.NotFoundError}
	}

	perk, perkExists := perks.GetPerk(payment.PerkId)
	if !perkExists {
		return common.RequestError{fmt.Sprintf("Perk not found %d", payment.PerkId), common.NotFoundError}
	}

	if updatePayment.Status == "failure" {
		message := "Failed status specified"
		log.Print(message)

		payment.UpdateStatus("failure")
		payment.UpdateFailureReason(message)

		paymentsCache.AddOrReplacePayment(updatePayment.payment)
		_, err := updatePaymentInDb(updatePayment.payment)
		if nil != err {
			log.Print(err)
			log.Printf("Error updating payment %s with information from processor", payment.Id)
		} else {
			log.Printf("Successfully updated payment %s in database", payment.Id)
		}

		return common.RequestError{message, common.BadRequestError}
	}

	executeResult, err := paypalClient.ExecuteApprovedPayment(updatePayment.PaypalPaymentId, updatePayment.PaypalPayerId)
	if nil == err {
		payment.UpdateStatus("success")
		advertisements.AddAdvertisementFromPayment(campaign.Name, payment)

		jsonStr, jsonErr := json.Marshal(executeResult)
		if nil == jsonErr {
			payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(string(jsonStr), "\"", "\\\"", -1))
		} else {
			log.Print(jsonErr)
			log.Printf("Unable to marshal payment response (%#v) from paypal", executeResult)
		}

		if campaignExists {
			campaign.IncrementAmtRaised(payment.Amount)
			campaign.IncrementNumBackers(1)
		}

		if perkExists {
			perk.IncrementNumClaimed(1)
		}
	} else {
		log.Print(err)
		payment.UpdateStatus("failure")

		jsonStr, jsonErr := json.Marshal(err)
		if nil == jsonErr {
			payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(string(jsonStr), "\"", "\\\"", -1))
		} else {
			log.Print(jsonErr)
			log.Printf("Unable to marshal payment response (%#v) from paypal", executeResult)
		}

		if nil != executeResult && len(executeResult.ID) == 0 {
			err = common.RequestError{err.Error(), common.BadRequestError}
		} else {
			err = common.RequestError{err.Error(), common.ServerError}
		}
	}

	paymentsCache.AddOrReplacePayment(updatePayment.payment)
	_, dbErr := updatePaymentInDb(updatePayment.payment)
	if nil != dbErr {
		log.Print(dbErr)
		log.Printf("Error updating payment %s with information from processor", payment.Id)
		log.Print("%#v", updatePayment.payment)
	} else {
		log.Printf("Successfully updated payment %s in database", payment.Id)
	}

	return err
}
