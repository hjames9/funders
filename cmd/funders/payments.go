package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/hjames9/funders"
	"github.com/martini-contrib/binding"
	"github.com/satori/go.uuid"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	GET_ACCOUNT_TYPES_QUERY  = "SELECT enum_range(NULL::funders.account_type) AS account_types"
	GET_PAYMENT_STATES_QUERY = "SELECT enum_range(NULL::funders.payment_state) AS payment_states"
	GET_PAYMENTS_QUERY       = "SELECT id, campaign_id, perk_id, pledge_id, account_type, state FROM funders.active_payments"
	GET_PAYMENT_QUERY        = "SELECT id, campaign_id, perk_id, pledge_id, account_type, state FROM funders.active_payments WHERE id = $1"
	ADD_PAYMENT_QUERY        = "INSERT INTO funders.payments(id, campaign_id, perk_id, account_type, name_on_payment, full_name, address1, address2, city, postal_code, country, amount, currency, state, contact_email, contact_opt_in, advertise, advertise_other, pledge_id, created_at, updated_at) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21) RETURNING id"
	EMAIL_REGEX              = "^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$"
	UUID_REGEX               = "^[a-z0-9]{8}-[a-z0-9]{4}-[1-5][a-z0-9]{3}-[a-z0-9]{4}-[a-z0-9]{12}$"
	PAYMENTS_URL             = "/payments"
)

type Payment struct {
	Id                        string
	CampaignId                int64 `form:"campaignId" binding:"required"`
	Campaign                  *Campaign
	PerkId                    int64 `form:"perkId" binding:"required"`
	Perk                      *Perk
	AccountType               string `form:"accountType" binding:"required"`
	NameOnPayment             string `form:"nameOnPayment" binding:"required"`
	CreditCardAccountNumber   string `form:"creditCardAccountNumber"`
	CreditCardExpirationDate  string `form:"creditCardExpirationDate"`
	CreditCardCvv             string `form:"creditCardCvv"`
	CreditCardPostalCode      string `form:"creditCardPostalCode"`
	PaypalRedirectUrl         string `form:"paypalRedirectUrl"`
	PaypalCancelUrl           string `form:"paypalCancelUrl"`
	PaypalApprovalUrl         string
	BitcoinAddress            string `form:"bitcoinAddress"`
	FullName                  string `form:"fullName" binding:"required"`
	Address1                  string `form:"address1" "binding:"required"`
	Address2                  string `form:"address2"`
	City                      string `form:"city" binding:"required"`
	PostalCode                string `form:"postalCode" binding:"required"`
	Country                   string `form:"country" "binding:"required"`
	Amount                    float64
	Currency                  string
	State                     string
	ContactEmail              string `form:"contactEmail"`
	ContactOptIn              bool   `form:"contactOptIn"`
	Advertise                 bool   `form:"advertise"`
	AdvertiseOther            string `form:"advertiseOther"`
	PaymentProcessorResponses string
	PaymentProcessorUsed      string
	FailureReason             string
	PledgeId                  string `form:"pledgeId"`
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

func (payment *Payment) UpdatePaypalApprovalUrl(paypalApprovalUrl string) string {
	payment.lock.Lock()
	defer payment.lock.Unlock()
	payment.PaypalApprovalUrl = paypalApprovalUrl
	return payment.PaypalApprovalUrl
}

func (payment *Payment) GetPaypalApprovalUrl() string {
	payment.lock.RLock()
	defer payment.lock.RUnlock()
	return payment.PaypalApprovalUrl
}

func (payment *Payment) MarshalJSON() ([]byte, error) {
	payment.lock.RLock()
	state := payment.State
	failureReason := payment.FailureReason
	paypalApprovalUrl := payment.PaypalApprovalUrl
	payment.lock.RUnlock()

	type MyPayment Payment
	return json.Marshal(&struct {
		Id                string    `json:"id"`
		CampaignId        int64     `json:"campaignId"`
		Campaign          *Campaign `json:"campaign"`
		PerkId            int64     `json:"perkId"`
		Perk              *Perk     `json:"perk"`
		State             string    `json:"state"`
		FailureReason     string    `json:"failureReason,omitempty"`
		PaypalApprovalUrl string    `json:"paypalApprovalUrl,omitempty"`
	}{
		Id:                payment.Id,
		CampaignId:        payment.CampaignId,
		Campaign:          payment.Campaign,
		PerkId:            payment.PerkId,
		Perk:              payment.Perk,
		State:             state,
		FailureReason:     failureReason,
		PaypalApprovalUrl: paypalApprovalUrl,
	})
}

func (payment *Payment) Validate(errors binding.Errors, req *http.Request) binding.Errors {
	errors = validateSizeLimit(payment.AccountType, "accountType", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.NameOnPayment, "nameOnPayment", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.CreditCardAccountNumber, "creditCardAccountNumber", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.CreditCardExpirationDate, "creditCardExpirationDate", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.CreditCardCvv, "creditCardCvv", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.CreditCardPostalCode, "creditCardPostalCode", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.PaypalRedirectUrl, "paypalRedirectUrl", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.PaypalCancelUrl, "paypalCancelUrl", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.BitcoinAddress, "bitcoinAddress", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.FullName, "fullName", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.Address1, "address1", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.Address2, "address2", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.City, "city", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.PostalCode, "postalCode", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.Country, "country", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.ContactEmail, "contactEmail", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.AdvertiseOther, "advertiseOther", stringSizeLimit, errors)
	errors = validateSizeLimit(payment.PledgeId, "pledgeId", stringSizeLimit, errors)

	if len(errors) == 0 {
		if !accountTypes[payment.AccountType] {
			message := fmt.Sprintf("Invalid account type \"%s\" specified", payment.AccountType)
			errors = addError(errors, []string{"accountType"}, binding.TypeError, message)
		}

		if payment.AccountType == "credit_card" && (len(payment.CreditCardAccountNumber) == 0 || len(payment.CreditCardExpirationDate) == 0 || len(payment.CreditCardCvv) == 0 || len(payment.CreditCardPostalCode) == 0) {
			errors = addError(errors, []string{"accountType", "creditCardAccountNumber", "creditCardExpirationDate", "creditCardCvv", "creditCardPostalCode"}, binding.RequiredError, "Credit card account number, expiration date, cvv and postal code required with credit_card account type")
		}

		if payment.AccountType == "paypal" && (len(payment.PaypalRedirectUrl) == 0 || len(payment.PaypalCancelUrl) == 0) {
			errors = addError(errors, []string{"accountType", "paypalRedirectUrl", "paypalCancelUrl"}, binding.RequiredError, "Paypal redirect and cancel url required with paypal account type")
		}

		if payment.AccountType == "bitcoin" && (len(payment.BitcoinAddress) == 0) {
			errors = addError(errors, []string{"accountType", "bitcoinAddress"}, binding.RequiredError, "Bitcoin address required with bitcoin account type")
		}

		if len(payment.ContactEmail) > 0 && !emailRegex.MatchString(payment.ContactEmail) {
			message := fmt.Sprintf("Invalid email \"%s\" format specified", payment.ContactEmail)
			errors = addError(errors, []string{"contactEmail"}, binding.TypeError, message)
		}

		perk, exists := perks.GetPerk(payment.PerkId)
		if exists {
			if !perk.IsAvailableForPayment() {
				message := fmt.Sprintf("Perk is not available. (%d/%d) claimed", perk.NumClaimed, perk.AvailableForPayment)
				errors = addError(errors, []string{"perkId"}, binding.TypeError, message)
			} else {
				payment.Amount = perk.Price
				payment.Currency = perk.Currency
				payment.Perk = perk
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
			} else {
				payment.Campaign = campaign
			}
		} else {
			message := fmt.Sprintf("Campaign not found with id: %d", payment.CampaignId)
			errors = addError(errors, []string{"campaignId"}, binding.TypeError, message)
		}

		if len(payment.PledgeId) > 0 {
			pledge, exists := pledges.GetPledge(payment.PledgeId)
			if !exists {
				message := fmt.Sprintf("Pledge not found with id: %d", payment.PledgeId)
				errors = addError(errors, []string{"pledgeId"}, binding.TypeError, message)
			} else if campaign.Id != pledge.CampaignId {
				message := fmt.Sprintf("Pledge %s on campaign %d does not match requested campaign id: %d", pledge.Id, pledge.CampaignId, payment.CampaignId)
				errors = addError(errors, []string{"pledgeId", "campaignId"}, binding.TypeError, message)
			} else if perk.Id != pledge.PerkId {
				message := fmt.Sprintf("Pledge %s on perk %d does not match requested perk id: %d", pledge.Id, pledge.PerkId, payment.PerkId)
				errors = addError(errors, []string{"pledgeId", "perkId"}, binding.TypeError, message)
			}

			_, exists = paymentsCache.GetPaymentByPledgeId(payment.PledgeId)
			if exists {
				message := fmt.Sprintf("Payment on pledge %s for perk %d already occurred", payment.PledgeId, payment.PerkId)
				errors = addError(errors, []string{"pledgeId"}, binding.TypeError, message)
			}
		} else {
			log.Print("No pledge id associated with payment")
		}

		if botDetection.IsBot(req) {
			message := "Go away spambot! We've alerted the authorities"
			errors = addError(errors, []string{"spambot"}, common.BOT_ERROR, message)
		}
	}

	return errors
}

type Payments struct {
	lock          sync.RWMutex
	paymentValues map[string]*Payment
	pledgeValues  map[string]*Payment
}

func NewPayments() *Payments {
	payments := new(Payments)
	payments.paymentValues = make(map[string]*Payment)
	payments.pledgeValues = make(map[string]*Payment)
	return payments
}

func (ps *Payments) AddOrReplacePayment(payment *Payment) *Payment {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	ps.paymentValues[payment.Id] = payment
	if len(payment.PledgeId) > 0 {
		ps.pledgeValues[payment.PledgeId] = payment
	}
	return payment
}

func (ps *Payments) AddOrReplacePayments(payments []*Payment) {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	for _, payment := range payments {
		ps.paymentValues[payment.Id] = payment
		if len(payment.PledgeId) > 0 {
			ps.pledgeValues[payment.PledgeId] = payment
		}
	}
}

func (ps *Payments) GetPayment(id string) (*Payment, bool) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	val, exists := ps.paymentValues[id]
	return val, exists
}

func (ps *Payments) GetPaymentByPledgeId(pledge_id string) (*Payment, bool) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	val, exists := ps.pledgeValues[pledge_id]
	return val, exists
}

//Payment enumerations
func getAccountTypes() string {
	var accountTypesStr string

	err := db.QueryRow(GET_ACCOUNT_TYPES_QUERY).Scan(&accountTypesStr)
	if nil != err {
		log.Print(err)
	} else {
		accountTypesStr = strings.Trim(accountTypesStr, "{}")
	}

	return accountTypesStr
}

func getPaymentStates() string {
	var paymentStatesStr string

	err := db.QueryRow(GET_PAYMENT_STATES_QUERY).Scan(&paymentStatesStr)
	if nil != err {
		log.Print(err)
	} else {
		paymentStatesStr = strings.Trim(paymentStatesStr, "{}")
	}

	return paymentStatesStr
}

//Used for validation
var emailRegex *regexp.Regexp
var uuidRegex *regexp.Regexp
var accountTypes map[string]bool
var paymentStates map[string]bool

//Asynchronous payments
var asyncPaymentRequest bool

//Background payment threads
var paymentBatchProcessor *common.BatchProcessor

func processPayment(payment *Payment) (error, int) {
	var err error
	var retCode int

	switch payment.AccountType {
	case "credit_card":
		fallthrough
	case "bitcoin":
		err = makeStripePayment(payment, nil)
	case "paypal":
		err = makePaypalPayment(payment, nil)
	default:
		err = common.RequestError{fmt.Sprintf("Unsupported payment account type %s", payment.AccountType), common.ServiceNotImplementedError}
	}

	if nil == err {
		//Success: StatusCreated (successful transaction from stripe and database)
		retCode = http.StatusCreated
	} else if err, found := err.(common.RequestError); found {
		switch common.RequestError(err).Type {
		case common.BadRequestError:
			//Error: StatusBadRequest (stripe or paypal couldn't process bad payment data)
			retCode = http.StatusBadRequest
		case common.NotFoundError:
			//Error: StatusNotFound(couldn't process previous payment)
			retCode = http.StatusNotFound
		case common.ServiceUnavailableError:
			//Error: StatusServiceUnavailable (stripe, paypal, or database is down)
			retCode = http.StatusServiceUnavailable
		case common.ServiceNotImplementedError:
			//Error: StatusNotImplemented (stripe, paypal or database is down)
			retCode = http.StatusNotImplemented
		case common.ServerError:
			fallthrough
		default:
			//Error: StatusInternalServerError (stripe, paypal or database had some processing error)
			retCode = http.StatusInternalServerError
		}
	} else {
		retCode = http.StatusInternalServerError
	}

	dbErr := addPayment(payment, nil)
	if nil != dbErr {
		log.Print(dbErr)
		log.Printf("%#v", payment)
	}

	return err, retCode
}

func processBatchPayment(paymentBatch []interface{}, waitGroup *sync.WaitGroup) {
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
	for _, paymentInterface := range paymentBatch {
		payment := paymentInterface.(*Payment)
		err = addPayment(payment, statement)
		if nil != err {
			log.Printf("Error processing payment %#v", payment)
			log.Print(err)
			continue
		}

		counter++
		switch payment.AccountType {
		case "credit_card":
			fallthrough
		case "bitcoin":
			waitGroup.Add(1)
			go makeStripePayment(payment, waitGroup)
		case "paypal":
			waitGroup.Add(1)
			go makePaypalPayment(payment, waitGroup)
		default:
			log.Printf("Unknown payment account type %s", payment.AccountType)
		}
	}

	err = transaction.Commit()
	if nil != err {
		log.Print("Error committing transaction")
		log.Print(err)
	} else {
		log.Printf("Processed %d payments", counter)
	}
}

func addPayment(payment *Payment, statement *sql.Stmt) error {
	var err error

	address2 := common.CreateSqlString(payment.Address2)
	contactEmail := common.CreateSqlString(payment.ContactEmail)
	advertiseOther := common.CreateSqlString(payment.AdvertiseOther)
	pledgeId := common.CreateSqlString(payment.PledgeId)

	if nil == statement {
		err = db.QueryRow(ADD_PAYMENT_QUERY, payment.Id, payment.CampaignId, payment.PerkId, payment.AccountType, payment.NameOnPayment, payment.FullName, payment.Address1, address2, payment.City, payment.PostalCode, payment.Country, payment.Amount, payment.Currency, payment.GetState(), contactEmail, payment.ContactOptIn, payment.Advertise, advertiseOther, pledgeId, time.Now(), time.Now()).Scan(&payment.Id)
	} else {
		err = statement.QueryRow(payment.Id, payment.CampaignId, payment.PerkId, payment.AccountType, payment.NameOnPayment, payment.FullName, payment.Address1, address2, payment.City, payment.PostalCode, payment.Country, payment.Amount, payment.Currency, payment.GetState(), contactEmail, payment.ContactOptIn, payment.Advertise, advertiseOther, pledgeId, time.Now(), time.Now()).Scan(&payment.Id)
	}

	if nil == err {
		log.Printf("New payment id = %s", payment.Id)
	}

	return err
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
		var pledgeId sql.NullString
		err = rows.Scan(&payment.Id, &payment.CampaignId, &payment.PerkId, &pledgeId, &payment.AccountType, &payment.State)
		if nil == err {
			if pledgeId.Valid {
				payment.PledgeId = pledgeId.String
			}
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
	var pledgeId sql.NullString
	err := db.QueryRow(GET_PAYMENT_QUERY, id).Scan(&payment.Id, &payment.CampaignId, &payment.PerkId, &pledgeId, &payment.AccountType, &payment.State)
	if pledgeId.Valid {
		payment.PledgeId = pledgeId.String
	}
	return payment, err
}

func getPayment(id string) (*Payment, error) {
	var err error
	payment, exists := paymentsCache.GetPayment(id)
	if !exists {
		var paymentDb Payment
		paymentDb, err = getPaymentFromDb(id)
		if nil == err {
			payment = paymentsCache.AddOrReplacePayment(&paymentDb)
			log.Print("Retrieved payment from database")
		} else {
			log.Print("Payment not found in database")
		}
	} else {
		log.Print("Retrieved payment from cache")
	}

	return payment, err
}

func getPaymentHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	req.Close = true

	var response common.Response
	id := strings.TrimSpace(req.URL.Query().Get("id"))

	if len(id) == 0 {
		responseStr := "Payment id parameter required"
		response = common.Response{Code: http.StatusBadRequest, Message: responseStr}
	} else if !uuidRegex.MatchString(id) {
		responseStr := fmt.Sprintf("Payment id parameter %s is in the wrong format", id)
		response = common.Response{Code: http.StatusBadRequest, Message: responseStr}
	} else {
		payment, err := getPayment(id)

		if sql.ErrNoRows == err {
			responseStr := fmt.Sprintf("%s not found", id)
			response = common.Response{Code: http.StatusNotFound, Message: responseStr}
			log.Print(err)
		} else if nil != err {
			responseStr := "Could not get payment due to server error"
			response = common.Response{Code: http.StatusInternalServerError, Message: responseStr}
			log.Print(err)
		} else {
			jsonStr, _ := json.Marshal(payment)
			return http.StatusOK, string(jsonStr)
		}
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
	var response common.Response

	if asyncPaymentRequest && nil != paymentBatchProcessor && paymentBatchProcessor.Running {
		paymentBatchProcessor.AddEvent(&payment)
		responseStr := "Successfully scheduled payment"
		response = common.Response{Code: http.StatusAccepted, Message: responseStr, Id: payment.Id}
		log.Print(responseStr)
	} else if !asyncPaymentRequest {
		err, retCode := processPayment(&payment)
		if nil != err {
			response = common.Response{Code: retCode, Message: err.Error(), Id: payment.Id}
			log.Print(err)
		} else {
			jsonStr, _ := json.Marshal(&payment)
			return retCode, string(jsonStr)
		}
	} else if asyncPaymentRequest && (nil == paymentBatchProcessor || !paymentBatchProcessor.Running) {
		responseStr := "Could not add payment due to server maintenance"
		response = common.Response{Code: http.StatusServiceUnavailable, Message: responseStr, Id: payment.Id}
		log.Print(responseStr)
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}
