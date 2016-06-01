package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hjames9/funders"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"log"
	"strconv"
	"strings"
	"time"
)

const (
	STRIPE_PROCESSOR = "stripe"
)

var stripeKey string

func makeStripePayment(payment *Payment) error {
	stripe.Key = stripeKey

	if payment.AccountType != "credit_card" {
		return errors.New("Only credit card payments are currently supported")
	}

	payment.PaymentProcessorUsed = STRIPE_PROCESSOR

	creditCardExpirationDate, err := time.Parse(common.TIME_LAYOUT, payment.CreditCardExpirationDate)
	if nil != err {
		return err
	}

	campaign, campaignExists := campaigns.GetCampaignById(payment.CampaignId)
	if !campaignExists {
		return errors.New(fmt.Sprintf("Campaign not found %d", payment.CampaignId))
	}

	perk, perkExists := perks.GetPerk(payment.PerkId)
	if !perkExists {
		return errors.New(fmt.Sprintf("Perk not found %d", payment.PerkId))
	}

	cardParams := &stripe.CardParams{
		Month:  strconv.Itoa(int(creditCardExpirationDate.Month())),
		Year:   strconv.Itoa(creditCardExpirationDate.Year()),
		Number: payment.CreditCardAccountNumber,
		CVC:    payment.CreditCardCvv,
		Name:   payment.NameOnPayment,
	}

	sourceParams := &stripe.SourceParams{
		Card: cardParams,
	}

	address := stripe.Address{
		Line1:   payment.Address1,
		Line2:   payment.Address2,
		City:    payment.City,
		Zip:     payment.PostalCode,
		Country: payment.Country,
	}

	shippingDetails := &stripe.ShippingDetails{
		Name:    payment.FullName,
		Address: address,
	}

	chargeParams := &stripe.ChargeParams{
		Amount:    uint64(perk.Price * 100), //Value is in cents
		Currency:  stripe.Currency(perk.Currency),
		Desc:      fmt.Sprintf("Payment id %d on charge for perk %d of campaign %d.", payment.Id, payment.PerkId, payment.CampaignId),
		Email:     payment.ContactEmail,
		Statement: fmt.Sprintf("Campaign(%s)", campaign.Name),
		Source:    sourceParams,
		Shipping:  shippingDetails,
	}

	ch, err := charge.New(chargeParams)
	if nil == err {
		log.Print("Successfully processed payment with processor")

		jsonStr, err := json.Marshal(ch)
		if nil == err {
			payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(string(jsonStr), "\"", "\\\"", -1))
		} else {
			log.Print(err)
			log.Printf("Unable to marshal charge response (%#v) from stripe", ch)
		}

		if ch.Paid {
			payment.UpdateState("success")
			if campaignExists {
				campaign.IncrementNumRaised(payment.Amount)
				campaign.IncrementNumBackers(1)
			}

			if perkExists {
				perk.IncrementNumClaimed(1)
			}
		} else {
			payment.UpdateState("failure")
			payment.UpdateFailureReason(ch.FailMsg)
		}
	} else {
		log.Print("Failed processing payment with processor")
		payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(err.Error(), "\"", "\\\"", -1))
		payment.UpdateState("failure")

		var stripeError = struct {
			Type    string
			Message string
			Code    string
			Status  int
		}{}
		err = json.Unmarshal([]byte(err.Error()), &stripeError)
		if nil == err {
			payment.UpdateFailureReason(stripeError.Message)
		} else {
			log.Print(err)
			log.Print("Error unmarshaling stripe error message")
		}
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
