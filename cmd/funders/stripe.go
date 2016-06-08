package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hjames9/funders"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/bitcoinreceiver"
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
	payment.PaymentProcessorUsed = STRIPE_PROCESSOR

	var sourceParams *stripe.SourceParams
	var bitcoinReceiver *stripe.BitcoinReceiver
	var err error

	if payment.AccountType == "credit_card" {
		sourceParams, err = makeStripeCreditCardPayment(payment)
		if nil != err {
			return err
		}
	} else if payment.AccountType == "bitcoin" {
		bitcoinReceiver, err = makeStripeBitcoinPayment(payment)
		if nil != err {
			return err
		}
	} else {
		return errors.New("Only credit card and bitcoin payments are currently supported")
	}

	campaign, campaignExists := campaigns.GetCampaignById(payment.CampaignId)
	if !campaignExists {
		return errors.New(fmt.Sprintf("Campaign not found %d", payment.CampaignId))
	}

	perk, perkExists := perks.GetPerk(payment.PerkId)
	if !perkExists {
		return errors.New(fmt.Sprintf("Perk not found %d", payment.PerkId))
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
		Desc:      fmt.Sprintf("Payment id %s on charge for perk %d of campaign %d.", payment.Id, payment.PerkId, payment.CampaignId),
		Email:     payment.ContactEmail,
		Statement: fmt.Sprintf("Campaign(%s)", campaign.Name),
		Source:    sourceParams,
		Shipping:  shippingDetails,
	}

	if nil != bitcoinReceiver {
		chargeParams.SetSource(bitcoinReceiver.ID)
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
				advertisements.AddAdvertisementFromPayment(campaign.Name, payment)
			}

			if perkExists {
				perk.IncrementNumClaimed(1)
			}
		} else {
			payment.UpdateState("failure")
			payment.UpdateFailureReason(ch.FailMsg)
		}
	} else {
		log.Print(err)
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

func makeStripeCreditCardPayment(payment *Payment) (*stripe.SourceParams, error) {
	creditCardExpirationDate, err := time.Parse(common.TIME_LAYOUT, payment.CreditCardExpirationDate)
	if nil != err {
		return nil, err
	}

	cardParams := &stripe.CardParams{
		Month:  strconv.Itoa(int(creditCardExpirationDate.Month())),
		Year:   strconv.Itoa(creditCardExpirationDate.Year()),
		Number: payment.CreditCardAccountNumber,
		CVC:    payment.CreditCardCvv,
		Name:   payment.NameOnPayment,
	}

	sourceParams := stripe.SourceParams{
		Card: cardParams,
	}

	return &sourceParams, nil
}

func makeStripeBitcoinPayment(payment *Payment) (*stripe.BitcoinReceiver, error) {
	receiverParams := &stripe.BitcoinReceiverParams{
		Amount:   uint64(payment.Amount * 100),      //Value is in cents
		Currency: stripe.Currency(payment.Currency), //Only USD supported for bitcoin
		Email:    payment.BitcoinAddress,
	}

	receiver, err := bitcoinreceiver.New(receiverParams)
	return receiver, err
}
