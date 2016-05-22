package main

import (
	"database/sql"
	"encoding/json"
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
	"sync"
	"time"
)

const (
	GET_PENDING_PAYMENTS_QUERY = "SELECT id, campaign_id, perk_id, state FROM funders.payments WHERE state = 'pending'"
	GET_PAYMENT_QUERY          = "SELECT id, campaign_id, perk_id, state FROM funders.payments WHERE id = $1"
	ADD_PAYMENT_QUERY          = "INSERT INTO funders.payments(id, campaign_id, perk_id, account_type, name_on_payment, bank_routing_number, bank_account_number, credit_card_account_number, credit_card_expiration_date, credit_card_cvv, credit_card_postal_code, paypal_email, bitcoin_address, full_name, address1, address2, city, postal_code, country, amount, state, contact_email, contact_opt_in, advertise, advertise_other, created_at, updated_at) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27) RETURNING id"
	UPDATE_PAYMENT_QUERY       = "UPDATE funders.payments SET updated_at = $1, state = $2, bank_routing_number = NULL, bank_account_number = NULL, credit_card_account_number = NULL, credit_card_expiration_date = NULL, credit_card_cvv = NULL, credit_card_postal_code = NULL, paypal_email = NULL, bitcoin_address = NULL WHERE id = $3 AND state <> 'pending'"
	EMAIL_REGEX                = "^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$"
	PAYMENTS_URL               = "/payments"
)

type Payment struct {
	Id                       string
	CampaignId               int64   `form:"campaignId" binding:"required"`
	PerkId                   int64   `form:"perkId" binding:"required"`
	AccountType              string  `form:"accountType" binding:"required" json:"-"`
	NameOnPayment            string  `form:"nameOnPayment" json:"-"`
	BankRoutingNumber        string  `form:"bankRoutingNumber" json:"-"`
	BankAccountNumber        string  `form:"bankAccountNumber" json:"-"`
	CreditCardAccountNumber  string  `form:"creditCardAccountNumber" json:"-"`
	CreditCardExpirationDate string  `form:"creditCardExpirationDate" json:"-"`
	CreditCardCvv            string  `form:"creditCardCvv" json:"-"`
	CreditCardPostalCode     string  `form:"creditCardPostalCode" json:"-"`
	PaypalEmail              string  `form:"paypalEmail" json:"-"`
	BitcoinAddress           string  `form:"bitcoinAddress" json:"-"`
	FullName                 string  `form:"fullName" json:"-"`
	Address1                 string  `form:"address1" json:"-"`
	Address2                 string  `form:"address2" json:"-"`
	City                     string  `form:"city" json:"-"`
	PostalCode               string  `form:"postalCode" json:"-"`
	Country                  string  `form:"country" json:"-"`
	Amount                   float64 `form:"amount" binding:"required" json:"-"`
	State                    string  `form:"state"`
	ContactEmail             string  `form:"contactEmail" json:"-"`
	ContactOptIn             bool    `form:"contactOptIn" json:"-"`
	Advertise                bool    `form:"advertise" json:"-"`
	AdvertiseOther           string  `form:"advertiseOther" json:"-"`
}

func (payment Payment) Validate(errors binding.Errors, req *http.Request) binding.Errors {
	errors = validateSizeLimit(payment.AccountType, "accountType", stringSizeLimit, errors)
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

func (ps Payments) AddOrReplacePayment(payment *Payment) *Payment {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	ps.values[payment.Id] = payment
	return payment
}

func (ps Payments) AddOrReplacePayments(payments []*Payment) {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	for _, payment := range payments {
		ps.values[payment.Id] = payment
	}
}

func (ps Payments) GetPayment(id string) (*Payment, bool) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	val, exists := ps.values[id]
	return val, exists
}

var emailRegex *regexp.Regexp
var asyncRequest bool
var payments chan Payment
var running bool
var waitGroup sync.WaitGroup

func processPayment(db *sql.DB, paymentBatch []Payment) {
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
		_, err = addPayment(db, payment, statement)
		if nil != err {
			log.Printf("Error processing payment %#v", payment)
			log.Print(err)
			continue
		}

		counter++
	}

	err = transaction.Commit()
	if nil != err {
		log.Print("Error committing transaction")
		log.Print(err)
	} else {
		log.Printf("Processed %d payments", counter)
	}
}

func batchAddPayment(db *sql.DB, asyncProcessInterval time.Duration, dbMaxOpenConns int) {
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
			go processPayment(db, elements[start:end])

			start = end
		}
	}
}

func makeStripePayment(payment Payment) error {
	stripe.Key = "sk_test_BQokikJOvBiI2HlWgH4olfQ2"

	chargeParams := &stripe.ChargeParams{
		Amount:   400,
		Currency: "usd",
		Desc:     "Charge for test@example.com",
	}

	chargeParams.SetSource("tok_189eV92eZvKYlo2CQy4JjX2D")
	_, err := charge.New(chargeParams)

	return err
}

func addPayment(db *sql.DB, payment Payment, statement *sql.Stmt) (string, error) {
	var lastInsertId string
	var err error

	bankRoutingNumber := common.CreateSqlString(payment.BankRoutingNumber)
	bankAccountNumber := common.CreateSqlString(payment.BankAccountNumber)
	creditCardAccountNumber := common.CreateSqlString(payment.CreditCardAccountNumber)
	creditCardExpirationDate := common.CreateSqlString(payment.CreditCardExpirationDate)
	creditCardCvv := common.CreateSqlString(payment.CreditCardCvv)
	creditCardPostalCode := common.CreateSqlString(payment.CreditCardPostalCode)
	paypalEmail := common.CreateSqlString(payment.PaypalEmail)
	bitcoinAddress := common.CreateSqlString(payment.BitcoinAddress)
	address2 := common.CreateSqlString(payment.Address2)
	contactEmail := common.CreateSqlString(payment.ContactEmail)
	advertiseOther := common.CreateSqlString(payment.AdvertiseOther)

	if nil == statement {
		err = db.QueryRow(ADD_PAYMENT_QUERY, payment.Id, payment.CampaignId, payment.PerkId, payment.AccountType, payment.NameOnPayment, bankRoutingNumber, bankAccountNumber, creditCardAccountNumber, creditCardExpirationDate, creditCardCvv, creditCardPostalCode, paypalEmail, bitcoinAddress, payment.FullName, payment.Address1, address2, payment.City, payment.PostalCode, payment.Country, payment.Amount, payment.State, contactEmail, payment.ContactOptIn, payment.Advertise, advertiseOther, time.Now(), time.Now()).Scan(&lastInsertId)
	} else {
		err = statement.QueryRow(payment.Id, payment.CampaignId, payment.PerkId, payment.AccountType, payment.NameOnPayment, bankRoutingNumber, bankAccountNumber, creditCardAccountNumber, creditCardExpirationDate, creditCardCvv, creditCardPostalCode, paypalEmail, bitcoinAddress, payment.FullName, payment.Address1, address2, payment.City, payment.PostalCode, payment.Country, payment.Amount, payment.State, contactEmail, payment.ContactOptIn, payment.Advertise, advertiseOther, time.Now(), time.Now()).Scan(&lastInsertId)
	}

	if nil == err {
		log.Printf("New payment id = %s", lastInsertId)
	}

	return lastInsertId, err
}

var paymentsCache = NewPayments()

func getPaymentsFromDb() ([]*Payment, error) {
	rows, err := db.Query(GET_PENDING_PAYMENTS_QUERY)
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

	if nil != err {
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
	payment.State = "pending"

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
		id, err := addPayment(db, payment, nil)
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
