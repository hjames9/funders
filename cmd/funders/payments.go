package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hjames9/funders"
	_ "github.com/lib/pq"
	"github.com/martini-contrib/binding"
	"github.com/satori/go.uuid"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	GET_PAYMENTS_QUERY   = "SELECT id, campaign_id, perk_id, state FROM funders.active_payments"
	GET_PAYMENT_QUERY    = "SELECT id, campaign_id, perk_id, state FROM funders.active_payments WHERE id = $1"
	ADD_PAYMENT_QUERY    = "INSERT INTO funders.payments(id, campaign_id, perk_id, account_type, name_on_payment, full_name, address1, address2, city, postal_code, country, amount, currency, state, contact_email, contact_opt_in, advertise, advertise_other, created_at, updated_at) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20) RETURNING id"
	UPDATE_PAYMENT_QUERY = "UPDATE funders.payments SET updated_at = $1, payment_processor_responses = payment_processor_responses || $2, payment_processor_used = $3, state = $4 WHERE id = $5"
	EMAIL_REGEX          = "^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$"
	PAYMENTS_URL         = "/payments"
	STRIPE_PROCESSOR     = "stripe"
)

type Payment struct {
	Id                        string
	CampaignId                int64   `form:"campaignId" binding:"required"`
	PerkId                    int64   `form:"perkId" binding:"required"`
	AccountType               string  `form:"accountType" binding:"required"`
	NameOnPayment             string  `form:"nameOnPayment" binding:"required"`
	BankRoutingNumber         string  `form:"bankRoutingNumber"`
	BankAccountNumber         string  `form:"bankAccountNumber"`
	CreditCardAccountNumber   string  `form:"creditCardAccountNumber"`
	CreditCardExpirationDate  string  `form:"creditCardExpirationDate"`
	CreditCardCvv             string  `form:"creditCardCvv"`
	CreditCardPostalCode      string  `form:"creditCardPostalCode"`
	PaypalEmail               string  `form:"paypalEmail"`
	BitcoinAddress            string  `form:"bitcoinAddress"`
	FullName                  string  `form:"fullName" binding:"required"`
	Address1                  string  `form:"address1" "binding:"required"`
	Address2                  string  `form:"address2"`
	City                      string  `form:"city" binding:"required"`
	PostalCode                string  `form:"postalCode" binding:"required"`
	Country                   string  `form:"country" "binding:"required"`
	Amount                    float64 `form:"amount" binding:"required"`
	Currency                  string  `form:"currency" binding:"required"`
	State                     string
	ContactEmail              string `form:"contactEmail"`
	ContactOptIn              bool   `form:"contactOptIn"`
	Advertise                 bool   `form:"advertise"`
	AdvertiseOther            string `form:"advertiseOther"`
	PaymentProcessorResponses string
	PaymentProcessorUsed      string
	FailureReason             string
	lock                      sync.RWMutex
}

func (payment *Payment) UpdateState(state string) string {
	payment.lock.Lock()
	defer payment.lock.Unlock()
	payment.State = state
	return payment.State
}

func (payment *Payment) GetState() string {
	payment.lock.RLock()
	defer payment.lock.RUnlock()
	return payment.State
}

func (payment *Payment) UpdateFailureReason(failureReason string) string {
	payment.lock.Lock()
	defer payment.lock.Unlock()
	payment.FailureReason = failureReason
	return payment.FailureReason
}

func (payment *Payment) GetFailureReason() string {
	payment.lock.RLock()
	defer payment.lock.RUnlock()
	return payment.FailureReason
}

func (payment *Payment) MarshalJSON() ([]byte, error) {
	payment.lock.RLock()
	state := payment.State
	failureReason := payment.FailureReason
	payment.lock.RUnlock()

	type MyPayment Payment
	return json.Marshal(&struct {
		Id            string `json:"id"`
		CampaignId    int64  `json:"campaignId"`
		PerkId        int64  `json:"perkId"`
		State         string `json:"state"`
		FailureReason string `json:"failureReason,omitempty"`
	}{
		Id:            payment.Id,
		CampaignId:    payment.CampaignId,
		PerkId:        payment.PerkId,
		State:         state,
		FailureReason: failureReason,
	})
}

func (payment *Payment) Validate(errors binding.Errors, req *http.Request) binding.Errors {
	errors = validateSizeLimit(payment.AccountType, "accountType", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.NameOnPayment, "nameOnPayment", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.BankRoutingNumber, "bankRoutingNumber", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.BankAccountNumber, "bankAccountNumber", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.CreditCardAccountNumber, "creditCardAccountNumber", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.CreditCardExpirationDate, "creditCardExpirationDate", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.CreditCardCvv, "creditCardCvv", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.CreditCardPostalCode, "creditCardPostalCode", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.PaypalEmail, "paypalEmail", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.BitcoinAddress, "bitcoinAddress", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.FullName, "fullName", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.Address1, "address1", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.Address2, "address2", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.City, "city", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.PostalCode, "postalCode", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.Country, "country", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.ContactEmail, "contactEmail", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.AdvertiseOther, "advertiseOther", stringSizeLimit, errors)

	if len(errors) == 0 {
		if !accountTypes[payment.AccountType] {
			message := fmt.Sprintf("Invalid account type \"%s\" specified", payment.AccountType)
			errors = addError(errors, []string{"accountType"}, binding.TypeError, message)
		}

		if payment.AccountType == "credit_card" && (len(payment.CreditCardAccountNumber) == 0 || len(payment.CreditCardExpirationDate) == 0 || len(payment.CreditCardCvv) == 0 || len(payment.CreditCardPostalCode) == 0) {
			errors = addError(errors, []string{"accountType", "creditCardAccountNumber", "creditCardExpirationDate", "creditCardCvv", "creditCardPostalCode"}, binding.RequiredError, "Credit card account number, expiration date, cvv and postal code required with credit_card account type")
		}

		if payment.AccountType == "bank_ach" && (len(payment.BankRoutingNumber) == 0 || len(payment.BankAccountNumber) == 0) {
			errors = addError(errors, []string{"accountType", "bankRoutingNumber", "bankAccountNumber"}, binding.RequiredError, "Bank routing number and account number required with bank_ach account type")
		}

		if payment.AccountType == "paypal" && (len(payment.PaypalEmail) == 0) {
			errors = addError(errors, []string{"accountType", "paypalEmail"}, binding.RequiredError, "Paypal email address required with paypal account type")
		}

		if payment.AccountType == "bitcoin" && (len(payment.BitcoinAddress) == 0) {
			errors = addError(errors, []string{"accountType", "bitcoinAddress"}, binding.RequiredError, "Bitcoin address required with bitcoin account type")
		}

		if len(payment.ContactEmail) > 0 && !emailRegex.MatchString(payment.ContactEmail) {
			message := fmt.Sprintf("Invalid email \"%s\" format specified", payment.ContactEmail)
			errors = addError(errors, []string{"contactEmail"}, binding.TypeError, message)
		}

		if len(payment.Currency) > 0 && currencies != nil && !currencies[payment.Currency] {
			message := fmt.Sprintf("Invalid currency \"%s\" specified", payment.Currency)
			errors = addError(errors, []string{"currency"}, binding.TypeError, message)
		}

		perk, exists := perks.GetPerk(payment.PerkId)
		if exists {
			if !perk.IsAvailable() {
				message := fmt.Sprintf("Perk is not available. (%d/%d) claimed", perk.Available, perk.numClaimed)
				errors = addError(errors, []string{"perkId"}, binding.TypeError, message)
			} else if perk.Price != payment.Amount {
				message := fmt.Sprintf("Payment amount (%f) does not match perk price (%f).", payment.Amount, perk.Price)
				errors = addError(errors, []string{"amount"}, binding.TypeError, message)
			} else if !strings.EqualFold(perk.Currency, payment.Currency) {
				message := fmt.Sprintf("Payment currency(%s) does not match perk currency (%s).", payment.Currency, perk.Currency)
				errors = addError(errors, []string{"currency"}, binding.TypeError, message)
			}
		} else {
			message := fmt.Sprintf("Perk not found with id: %d for campaign: %d", payment.PerkId, payment.CampaignId)
			errors = addError(errors, []string{"perkId"}, binding.TypeError, message)
		}

		campaign, exists := campaigns.GetCampaignById(payment.CampaignId)
		if exists {
			if campaign.HasEnded() {
				message := fmt.Sprintf("Campaign %s with id: %d has expired on %s", campaign.Name, payment.CampaignId, campaign.EndDate)
				errors = addError(errors, []string{"campaignId"}, binding.TypeError, message)
			} else if !campaign.HasStarted() {
				message := fmt.Sprintf("Campaign %s with id: %d will start on %s", campaign.Name, payment.CampaignId, campaign.StartDate)
				errors = addError(errors, []string{"campaignId"}, binding.TypeError, message)
			}
		} else {
			message := fmt.Sprintf("Campaign not found with id: %d", payment.CampaignId)
			errors = addError(errors, []string{"campaignId"}, binding.TypeError, message)
		}
	}

	return errors
}

type Payments struct {
	lock   sync.RWMutex
	values map[string]*Payment
}

func NewPayments() *Payments {
	payments := new(Payments)
	payments.values = make(map[string]*Payment)
	return payments
}

func (ps *Payments) AddOrReplacePayment(payment *Payment) *Payment {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	ps.values[payment.Id] = payment
	return payment
}

func (ps *Payments) AddOrReplacePayments(payments []*Payment) {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	for _, payment := range payments {
		ps.values[payment.Id] = payment
	}
}

func (ps *Payments) GetPayment(id string) (*Payment, bool) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	val, exists := ps.values[id]
	return val, exists
}

var currencies map[string]bool
var emailRegex *regexp.Regexp
var asyncRequest bool
var payments chan Payment
var running bool
var waitGroup sync.WaitGroup
var paymentProcessorKey string

func processPayment(paymentBatch []Payment) {
	log.Printf("Starting batch processing of %d payments", len(paymentBatch))

	defer waitGroup.Done()

	transaction, err := db.Begin()
	if nil != err {
		log.Print("Error creating transaction")
		log.Print(err)
	}

	defer transaction.Rollback()
	statement, err := transaction.Prepare(ADD_PAYMENT_QUERY)
	if nil != err {
		log.Print("Error preparing SQL statement")
		log.Print(err)
	}

	defer statement.Close()

	counter := 0
	for _, payment := range paymentBatch {
		_, err = addPayment(payment, statement)
		if nil != err {
			log.Printf("Error processing payment %#v", payment)
			log.Print(err)
			continue
		}

		counter++
		go makeStripePayment(&payment)
	}

	err = transaction.Commit()
	if nil != err {
		log.Print("Error committing transaction")
		log.Print(err)
	} else {
		log.Printf("Processed %d payments", counter)
	}
}

func batchAddPayment(asyncProcessInterval time.Duration, dbMaxOpenConns int) {
	log.Print("Started batch writing thread")

	defer waitGroup.Done()

	for running {
		time.Sleep(asyncProcessInterval * time.Second)

		var elements []Payment
		processing := true
		for processing {
			select {
			case payment, ok := <-payments:
				if ok {
					elements = append(elements, payment)
					break
				} else {
					log.Print("Select channel closed")
					processing = false
					running = false
					break
				}
			default:
				processing = false
				break
			}
		}

		if len(elements) <= 0 {
			continue
		}

		log.Printf("Retrieved %d payments.  Processing with %d connections", len(elements), dbMaxOpenConns)

		sliceSize := int(math.Floor(float64(len(elements) / dbMaxOpenConns)))
		remainder := len(elements) % dbMaxOpenConns
		start := 0
		end := 0

		for iter := 0; iter < dbMaxOpenConns; iter++ {
			var leftover int
			if remainder > 0 {
				leftover = 1
				remainder--
			} else {
				leftover = 0
			}

			end += sliceSize + leftover

			if start == end {
				break
			}

			waitGroup.Add(1)
			go processPayment(elements[start:end])

			start = end
		}
	}
}

func makeStripePayment(payment *Payment) error {
	stripe.Key = paymentProcessorKey

	if payment.AccountType != "credit_card" {
		return errors.New("Only credit card payments are currently supported")
	}

	payment.PaymentProcessorUsed = STRIPE_PROCESSOR

	layout := "2006-01-02"
	creditCardExpirationDate, err := time.Parse(layout, payment.CreditCardExpirationDate)
	if nil != err {
		return err
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

	chargeParams := &stripe.ChargeParams{
		Amount:   uint64(payment.Amount * 100), //Value is in cents
		Currency: stripe.Currency(payment.Currency),
		Desc:     fmt.Sprintf("Payment id %d on charge for perk %d of campaign %d.", payment.Id, payment.PerkId, payment.CampaignId),
		Source:   sourceParams,
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
			campaign, exists := campaigns.GetCampaignById(payment.CampaignId)
			if exists {
				campaign.IncrementNumRaised(payment.Amount)
				campaign.IncrementNumBackers(1)
			}

			perk, exists := perks.GetPerk(payment.PerkId)
			if exists {
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
	_, err = db.Exec(UPDATE_PAYMENT_QUERY, time.Now(), payment.PaymentProcessorResponses, payment.PaymentProcessorUsed, payment.GetState(), payment.Id)
	if nil != err {
		log.Print(err)
		log.Printf("Error updating payment %s with information from processor", payment.Id)
	} else {
		log.Printf("Successfully updated payment %s in database", payment.Id)
	}

	return err
}

func addPayment(payment Payment, statement *sql.Stmt) (string, error) {
	var lastInsertId string
	var err error

	address2 := common.CreateSqlString(payment.Address2)
	contactEmail := common.CreateSqlString(payment.ContactEmail)
	advertiseOther := common.CreateSqlString(payment.AdvertiseOther)

	if nil == statement {
		err = db.QueryRow(ADD_PAYMENT_QUERY, payment.Id, payment.CampaignId, payment.PerkId, payment.AccountType, payment.NameOnPayment, payment.FullName, payment.Address1, address2, payment.City, payment.PostalCode, payment.Country, payment.Amount, payment.Currency, payment.GetState(), contactEmail, payment.ContactOptIn, payment.Advertise, advertiseOther, time.Now(), time.Now()).Scan(&lastInsertId)
	} else {
		err = statement.QueryRow(payment.Id, payment.CampaignId, payment.PerkId, payment.AccountType, payment.NameOnPayment, payment.FullName, payment.Address1, address2, payment.City, payment.PostalCode, payment.Country, payment.Amount, payment.Currency, payment.GetState(), contactEmail, payment.ContactOptIn, payment.Advertise, advertiseOther, time.Now(), time.Now()).Scan(&lastInsertId)
	}

	if nil == err {
		log.Printf("New payment id = %s", lastInsertId)
	}

	return lastInsertId, err
}

var paymentsCache = NewPayments()

func getPaymentsFromDb() ([]*Payment, error) {
	rows, err := db.Query(GET_PAYMENTS_QUERY)
	if nil != err {
		return nil, err
	}

	defer rows.Close()

	var payments []*Payment
	for rows.Next() {
		var payment Payment
		err = rows.Scan(&payment.Id, &payment.CampaignId, &payment.PerkId, &payment.State)
		if nil == err {
			payments = append(payments, &payment)
		} else {
			break
		}
	}

	if nil == err {
		err = rows.Err()
	}

	return payments, err
}

func getPaymentFromDb(id string) (Payment, error) {
	var payment Payment
	err := db.QueryRow(GET_PAYMENT_QUERY, id).Scan(&payment.Id, &payment.CampaignId, &payment.PerkId, &payment.State)
	return payment, err
}

func getPayment(id string) (*Payment, error) {
	var err error
	payment, exists := paymentsCache.GetPayment(id)
	if !exists {
		var paymentDb Payment
		paymentDb, err = getPaymentFromDb(id)
		payment = paymentsCache.AddOrReplacePayment(&paymentDb)
		log.Print("Retrieved payment from database")
	} else {
		log.Print("Retrieved payment from cache")
	}

	return payment, err
}

func getPaymentHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	req.Close = true

	var response Response
	id := req.URL.Query().Get("id")

	payment, err := getPayment(id)

	if sql.ErrNoRows == err {
		responseStr := fmt.Sprintf("%s not found", id)
		response = Response{Code: http.StatusNotFound, Message: responseStr}
		log.Print(err)
	} else if nil != err {
		responseStr := "Could not get payment due to server error"
		response = Response{Code: http.StatusInternalServerError, Message: responseStr}
		log.Print(err)
	} else {
		jsonStr, _ := json.Marshal(payment)
		return http.StatusOK, string(jsonStr)
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}

func makePaymentHandler(res http.ResponseWriter, req *http.Request, payment Payment) (int, string) {
	payment.Id = uuid.NewV4().String()
	payment.UpdateState("pending")

	paymentsCache.AddOrReplacePayment(&payment)
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	res.Header().Set(LOCATION_HEADER, fmt.Sprintf("%s?id=%s", PAYMENTS_URL, payment.Id))

	log.Printf("Received new payment: %#v", payment)

	req.Close = true
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	var response Response

	if asyncRequest && running {
		payments <- payment
		responseStr := "Successfully added payment"
		response = Response{Code: http.StatusAccepted, Message: responseStr, Id: payment.Id}
		log.Print(responseStr)
	} else if asyncRequest && !running {
		responseStr := "Could not add payment due to server maintenance"
		response = Response{Code: http.StatusServiceUnavailable, Message: responseStr, Id: payment.Id}
		log.Print(responseStr)
	} else {
		id, err := addPayment(payment, nil)
		if nil != err {
			responseStr := "Could not add payment due to server error"
			response = Response{Code: http.StatusInternalServerError, Message: responseStr, Id: payment.Id}
			log.Print(responseStr)
			log.Print(err)
			log.Printf("%d database connections opened", db.Stats().OpenConnections)
		} else {
			responseStr := "Successfully added payment"
			response = Response{Code: http.StatusCreated, Message: responseStr, Id: id}
			log.Print(responseStr)
		}
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}
