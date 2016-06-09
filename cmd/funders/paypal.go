package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/logpacker/PayPal-Go-SDK"
	"log"
	"strconv"
	"strings"
	"sync"
)

const (
	PAYPAL_PROCESSOR = "paypal"
)

var paypalClient *paypalsdk.Client

func makePaypalPayment(payment *Payment, waitGroup *sync.WaitGroup) error {
	defer waitGroup.Done()

	if payment.AccountType != "paypal" {
		return errors.New("Only paypal payments are currently supported")
	}

	payment.PaymentProcessorUsed = PAYPAL_PROCESSOR

	campaign, campaignExists := campaigns.GetCampaignById(payment.CampaignId)
	if !campaignExists {
		return errors.New(fmt.Sprintf("Campaign not found %d", payment.CampaignId))
	}

	perk, perkExists := perks.GetPerk(payment.PerkId)
	if !perkExists {
		return errors.New(fmt.Sprintf("Perk not found %d", payment.PerkId))
	}

	amount := paypalsdk.Amount{
		Total:    strconv.FormatFloat(payment.Amount, 'f', -1, 64),
		Currency: payment.Currency,
	}

	redirectURI := fmt.Sprintf("%s&paymentId=%s", payment.PaypalRedirectUrl, payment.Id)
	cancelURI := fmt.Sprintf("%s&paymentId=%s", payment.PaypalCancelUrl, payment.Id)
	description := fmt.Sprintf("Perk(%s) for Campaign(%s). Payment id(%s)", perk.Name, campaign.Name, payment.Id)
	paymentResult, err := paypalClient.CreateDirectPaypalPayment(amount, redirectURI, cancelURI, description)

	if nil == err {
		jsonStr, err := json.Marshal(paymentResult)
		if nil == err {
			payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(string(jsonStr), "\"", "\\\"", -1))
		} else {
			log.Print(err)
			log.Printf("Unable to marshal payment response (%#v) from paypal", paymentResult)
		}

		//Set approval url
		payment.PaypalApprovalUrl = paymentResult.Links[0].Href
	} else {
		log.Printf("%#v", err)
		log.Print("Failed processing payment with processor")

		jsonStr, err := json.Marshal(err)
		if nil != err {
			log.Print(err)
			log.Print("Unable to marshal paypal error")
		} else {
			payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(string(jsonStr), "\"", "\\\"", -1))
		}

		payment.UpdateState("failure")
	}

	paymentsCache.AddOrReplacePayment(payment)
	_, err = updatePaymentInDb(payment)
	if nil != err {
		log.Print(err)
		log.Printf("Error updating payment %s with information from processor", payment.Id)
	} else {
		log.Printf("Successfully updated payment %s in database", payment.Id)
	}

	return err
}

func executePaypalPayment(updatePayment *UpdatePayment, waitGroup *sync.WaitGroup) error {
	defer waitGroup.Done()

	payment := updatePayment.payment

	campaign, campaignExists := campaigns.GetCampaignById(payment.CampaignId)
	if !campaignExists {
		return errors.New(fmt.Sprintf("Campaign not found %d", payment.CampaignId))
	}

	perk, perkExists := perks.GetPerk(payment.PerkId)
	if !perkExists {
		return errors.New(fmt.Sprintf("Perk not found %d", payment.PerkId))
	}

	if updatePayment.State == "failure" {
		message := "Failed state specified"
		log.Print(message)

		payment.UpdateState("failure")
		payment.UpdateFailureReason(message)

		paymentsCache.AddOrReplacePayment(updatePayment.payment)
		_, err := updatePaymentInDb(updatePayment.payment)
		if nil != err {
			log.Print(err)
			log.Printf("Error updating payment %s with information from processor", payment.Id)
		} else {
			log.Printf("Successfully updated payment %s in database", payment.Id)
		}

		return errors.New(message)
	}

	executeResult, err := paypalClient.ExecuteApprovedPayment(updatePayment.PaypalPaymentId, updatePayment.PaypalPayerId)
	if nil == err {
		payment.UpdateState("success")
		advertisements.AddAdvertisementFromPayment(campaign.Name, payment)

		jsonStr, err := json.Marshal(executeResult)
		if nil == err {
			payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(string(jsonStr), "\"", "\\\"", -1))
		} else {
			log.Print(err)
			log.Printf("Unable to marshal payment response (%#v) from paypal", executeResult)
		}

		if campaignExists {
			campaign.IncrementNumRaised(payment.Amount)
			campaign.IncrementNumBackers(1)
		}

		if perkExists {
			perk.IncrementNumClaimed(1)
		}
	} else {
		log.Print(err)
		payment.UpdateState("failure")

		jsonStr, err := json.Marshal(err)
		if nil == err {
			payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(string(jsonStr), "\"", "\\\"", -1))
		} else {
			log.Print(err)
			log.Printf("Unable to marshal payment response (%#v) from paypal", executeResult)
		}
	}

	paymentsCache.AddOrReplacePayment(updatePayment.payment)
	_, err = updatePaymentInDb(updatePayment.payment)
	if nil != err {
		log.Print(err)
		log.Printf("Error updating payment %s with information from processor", payment.Id)
	} else {
		log.Printf("Successfully updated payment %s in database", payment.Id)
	}

	return err
}
