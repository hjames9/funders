package main

import (
	"encoding/json"
	"fmt"
	"github.com/hjames9/funders"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/bitcoinreceiver"
	"github.com/stripe/stripe-go/charge"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	STRIPE_PROCESSOR = "stripe"
)

var stripeKey string

func makeStripePayment(payment *Payment, waitGroup *sync.WaitGroup) error {
	if nil != waitGroup {
		defer waitGroup.Done()
	}

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
		return common.RequestError{"Only credit card and bitcoin payments are currently supported", common.ServiceNotImplementedError}
	}

	campaign, campaignExists := campaigns.GetCampaignById(payment.CampaignId)
	if !campaignExists {
		return common.RequestError{fmt.Sprintf("Campaign not found %d", payment.CampaignId), common.NotFoundError}
	}

	perk, perkExists := perks.GetPerk(payment.PerkId)
	if !perkExists {
		return common.RequestError{fmt.Sprintf("Perk not found %d", payment.PerkId), common.NotFoundError}
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

		jsonStr, jsonErr := json.Marshal(ch)
		if nil == jsonErr {
			payment.PaymentProcessorResponses = fmt.Sprintf("{\"%s\"}", strings.Replace(string(jsonStr), "\"", "\\\"", -1))
		} else {
			log.Print(jsonErr)
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

		if stripeErr, ok := err.(*stripe.Error); ok {
			switch stripeErr.Code {
			case stripe.IncorrectNum:
				fallthrough
			case stripe.InvalidNum:
				fallthrough
			case stripe.InvalidExpM:
				fallthrough
			case stripe.InvalidExpY:
				fallthrough
			case stripe.InvalidCvc:
				fallthrough
			case stripe.ExpiredCard:
				fallthrough
			case stripe.IncorrectCvc:
				fallthrough
			case stripe.IncorrectZip:
				fallthrough
			case stripe.CardDeclined:
				fallthrough
			case stripe.Missing:
				fallthrough
			case stripe.ProcessingErr:
				err = common.RequestError{stripeErr.Msg, common.BadRequestError}
			case stripe.RateLimit:
				fallthrough
			default:
				err = common.RequestError{stripeErr.Msg, common.ServerError}
			}

			payment.UpdateFailureReason(stripeErr.Msg)
		} else {
			errorMsg := "Really bad Server error"
			payment.UpdateFailureReason(errorMsg)
			err = common.RequestError{errorMsg, common.ServerError}
		}
	}

	paymentsCache.AddOrReplacePayment(payment)
	if nil != waitGroup {
		_, dbErr := updatePaymentInDb(payment)
		if nil != dbErr {
			log.Print(dbErr)
			log.Printf("Error updating payment %s with information from processor", payment.Id)
		} else {
			log.Printf("Successfully updated payment %s in database", payment.Id)
		}
	}

	return err
}

func makeStripeCreditCardPayment(payment *Payment) (*stripe.SourceParams, error) {
	creditCardExpirationDate, err := time.Parse(common.TIME_LAYOUT, payment.CreditCardExpirationDate)
	if nil != err {
		return nil, common.RequestError{err.Error(), common.BadRequestError}
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
	if nil != err {
		err = common.RequestError{err.Error(), common.BadRequestError}
	}

	return receiver, err
}
