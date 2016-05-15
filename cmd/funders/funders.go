package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/hjames9/funders"
	_ "github.com/lib/pq"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/cors"
	"github.com/martini-contrib/secure"
	"github.com/satori/go.uuid"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	GET_CAMPAIGNS_QUERY  = "SELECT id, name, description, goal, num_raised, num_backers, start_date, end_date, flexible FROM funders.campaign_backers WHERE active = true"
	GET_CAMPAIGN_QUERY   = "SELECT id, name, description, goal, num_raised, num_backers, start_date, end_date, flexible FROM funders.campaign_backers WHERE active = true AND name = $1"
	GET_PERKS_QUERY      = "SELECT id, campaign_id, campaign_name, name, description, price, available, ship_date, num_claimed FROM funders.perk_claims WHERE active = true AND campaign_name = $1"
	GET_PAYMENT_QUERY    = "SELECT id, campaign_id, perk_id, state FROM funders.payments WHERE id = $1"
	ADD_PAYMENT_QUERY    = "INSERT INTO funders.payments(id, campaign_id, perk_id, account_type, name_on_payment, bank_routing_number, bank_account_number, credit_card_account_number, credit_card_expiration_date, credit_card_cvv, credit_card_postal_code, paypal_email, bitcoin_address, full_name, address1, address2, city, postal_code, country, amount, state, contact_email, contact_opt_in, advertise, advertise_other, created_at, updated_at) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27) RETURNING id"
	UPDATE_PAYMENT_QUERY = "UPDATE funders.payments SET updated_at = $1, state = $2, bank_routing_number = NULL, bank_account_number = NULL, credit_card_account_number = NULL, credit_card_expiration_date = NULL, credit_card_cvv = NULL, credit_card_postal_code = NULL, paypal_email = NULL, bitcoin_address = NULL WHERE id = $3 AND state <> 'pending'"
	EMAIL_REGEX          = "^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$"
	CAMPAIGN_URL         = "/campaigns"
	PERKS_URL            = "/perks"
	PAYMENTS_URL         = "/payments"
	USER_AGENT_HEADER    = "User-Agent"
	CONTENT_TYPE_HEADER  = "Content-Type"
	LOCATION_HEADER      = "Location"
	ORIGIN_HEADER        = "Origin"
	JSON_CONTENT_TYPE    = "application/json"
	BOT_ERROR            = "BotError"
	GET_METHOD           = "GET"
	POST_METHOD          = "POST"
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

type Response struct {
	Code    int
	Message string
	Id      string `json:",omitempty"`
}

type RequestLocation int

const (
	Header RequestLocation = 1 << iota
	Body
)

type Position int

const (
	First Position = 1 << iota
	Last
)

type BotDetection struct {
	FieldLocation RequestLocation
	FieldName     string
	FieldValue    string
	MustMatch     bool
	PlayCoy       bool
}

func (botDetection BotDetection) IsBot(req *http.Request) bool {
	var botField string

	switch botDetection.FieldLocation {
	case Header:
		botField = req.Header.Get(botDetection.FieldName)
		break
	case Body:
		botField = req.FormValue(botDetection.FieldName)
		break
	}

	if botDetection.MustMatch && botDetection.FieldValue == botField {
		return false
	} else if !botDetection.MustMatch && botDetection.FieldValue != botField {
		return false
	}

	return true
}

var stringSizeLimit int
var emailRegex *regexp.Regexp
var botDetection BotDetection

func (payment Payment) Validate(errors binding.Errors, req *http.Request) binding.Errors {
	errors = validateSizeLimit(payment.AccountType, "accountType", stringSizeLimit, errors)
	return errors
}

func validateSizeLimit(field string, fieldName string, sizeLimit int, errors binding.Errors) binding.Errors {
	if len(field) > sizeLimit {
		message := fmt.Sprintf("Field %s size %d is too large", fieldName, len(field))
		errors = addError(errors, []string{fieldName}, binding.TypeError, message)
	}
	return errors
}

func addError(errors binding.Errors, fieldNames []string, classification string, message string) binding.Errors {
	errors = append(errors, binding.Error{
		FieldNames:     fieldNames,
		Classification: classification,
		Message:        message,
	})
	return errors
}

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

func createSqlString(value string) sql.NullString {
	var nullValue sql.NullString
	if len(value) != 0 {
		nullValue = sql.NullString{value, true}
	}
	return nullValue
}

func addPayment(db *sql.DB, payment Payment, statement *sql.Stmt) (string, error) {
	var lastInsertId string
	var err error

	bankRoutingNumber := createSqlString(payment.BankRoutingNumber)
	bankAccountNumber := createSqlString(payment.BankAccountNumber)
	creditCardAccountNumber := createSqlString(payment.CreditCardAccountNumber)
	creditCardExpirationDate := createSqlString(payment.CreditCardExpirationDate)
	creditCardCvv := createSqlString(payment.CreditCardCvv)
	creditCardPostalCode := createSqlString(payment.CreditCardPostalCode)
	paypalEmail := createSqlString(payment.PaypalEmail)
	bitcoinAddress := createSqlString(payment.BitcoinAddress)
	address2 := createSqlString(payment.Address2)
	contactEmail := createSqlString(payment.ContactEmail)
	advertiseOther := createSqlString(payment.AdvertiseOther)

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

var db *sql.DB

func getCampaignHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	req.Close = true

	var campaign common.Campaign
	var response Response
	campaignName := req.URL.Query().Get("name")

	err := db.QueryRow(GET_CAMPAIGN_QUERY, campaignName).Scan(&campaign.Id, &campaign.Name, &campaign.Description, &campaign.Goal, &campaign.NumRaised, &campaign.NumBackers, &campaign.StartDate, &campaign.EndDate, &campaign.Flexible)

	if sql.ErrNoRows == err {
		responseStr := fmt.Sprintf("%s not found", campaignName)
		response = Response{Code: http.StatusNotFound, Message: responseStr}
		log.Print(responseStr)
	} else if nil != err {
		responseStr := "Could not get campaign due to server error"
		response = Response{Code: http.StatusInternalServerError, Message: responseStr}
		log.Print(err)
	} else {
		jsonStr, _ := json.Marshal(campaign)
		return http.StatusOK, string(jsonStr)
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}

func getPerkHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	req.Close = true

	var response Response
	campaignName := req.URL.Query().Get("campaign_name")

	rows, err := db.Query(GET_PERKS_QUERY, campaignName)
	defer rows.Close()

	if nil != err {
		responseStr := "Could not get perks due to server error"
		response = Response{Code: http.StatusInternalServerError, Message: responseStr}
		log.Print(err)
	} else {
		var perks []common.Perk
		for rows.Next() {
			var perk common.Perk
			err = rows.Scan(&perk.Id, &perk.CampaignId, &perk.CampaignName, &perk.Name, &perk.Description, &perk.Price, &perk.Available, &perk.ShipDate, &perk.NumClaimed)
			if nil == err {
				perks = append(perks, perk)
			} else {
				responseStr := "Could not get perks due to server error"
				response = Response{Code: http.StatusInternalServerError, Message: responseStr}
				log.Print(err)
			}
		}

		err = rows.Err()
		if nil != err {
			log.Print(err)
		}

		if len(perks) > 0 {
			jsonStr, _ := json.Marshal(perks)
			return http.StatusOK, string(jsonStr)
		} else {
			responseStr := fmt.Sprintf("%s not found", campaignName)
			response = Response{Code: http.StatusNotFound, Message: responseStr}
			log.Print(responseStr)
		}
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}

func getPaymentHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	req.Close = true

	var payment Payment
	var response Response
	id := req.URL.Query().Get("id")

	err := db.QueryRow(GET_PAYMENT_QUERY, id).Scan(&payment.Id, &payment.CampaignId, &payment.PerkId, &payment.State)

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

func errorHandler(errors binding.Errors, res http.ResponseWriter) {
	if len(errors) > 0 {
		var fieldsMsg string

		for _, err := range errors {
			for _, field := range err.Fields() {
				fieldsMsg += fmt.Sprintf("%s, ", field)
			}

			log.Printf("Error received. Message: %s, Kind: %s", err.Error(), err.Kind())
		}

		fieldsMsg = strings.TrimSuffix(fieldsMsg, ", ")

		log.Printf("Error received. Fields: %s", fieldsMsg)

		res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
		var response Response

		if errors.Has(binding.RequiredError) {
			res.WriteHeader(http.StatusBadRequest)
			responseStr := fmt.Sprintf("Missing required field(s): %s", fieldsMsg)
			response = Response{Code: http.StatusBadRequest, Message: responseStr}
		} else if errors.Has(binding.ContentTypeError) {
			res.WriteHeader(http.StatusUnsupportedMediaType)
			response = Response{Code: http.StatusUnsupportedMediaType, Message: "Invalid content type"}
		} else if errors.Has(binding.DeserializationError) {
			res.WriteHeader(http.StatusBadRequest)
			response = Response{Code: http.StatusBadRequest, Message: "Deserialization error"}
		} else if errors.Has(binding.TypeError) {
			res.WriteHeader(http.StatusBadRequest)
			response = Response{Code: http.StatusBadRequest, Message: errors[0].Error()}
		} else if errors.Has(BOT_ERROR) {
			if botDetection.PlayCoy && !asyncRequest {
				res.WriteHeader(http.StatusCreated)
				//response = Response{Code: http.StatusCreated, Message: "Successfully added payment", Id: getNextId(db)}
				log.Printf("Robot detected: %s. Playing coy.", errors[0].Error())
			} else if botDetection.PlayCoy && asyncRequest {
				res.WriteHeader(http.StatusAccepted)
				response = Response{Code: http.StatusAccepted, Message: "Successfully added payment"}
				log.Printf("Robot detected: %s. Playing coy.", errors[0].Error())
			} else {
				res.WriteHeader(http.StatusBadRequest)
				response = Response{Code: http.StatusBadRequest, Message: errors[0].Error()}
				log.Printf("Robot detected: %s. Rejecting message.", errors[0].Error())
			}
		} else {
			res.WriteHeader(http.StatusBadRequest)
			response = Response{Code: http.StatusBadRequest, Message: "Unknown error"}
		}

		log.Print(response.Message)
		jsonStr, _ := json.Marshal(response)
		res.Write(jsonStr)
	}
}

func notFoundHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	req.Close = true
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	responseStr := fmt.Sprintf("URL Not Found %s", req.URL)
	response := Response{Code: http.StatusNotFound, Message: responseStr}
	log.Print(responseStr)
	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}

func runHttpServer() {
	martini_ := martini.Classic()

	allowHeaders := []string{ORIGIN_HEADER}
	if botDetection.FieldLocation == Header {
		allowHeaders = append(allowHeaders, botDetection.FieldName)
	}

	//Allowable header names
	headerNamesStr := os.Getenv("ALLOW_HEADERS")
	if len(headerNamesStr) > 0 {
		headerNamesArr := strings.Split(headerNamesStr, ",")
		for _, headerName := range headerNamesArr {
			allowHeaders = append(allowHeaders, headerName)
		}
	}

	log.Printf("Allowable header names: %s", allowHeaders)

	martini_.Use(cors.Allow(&cors.Options{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{GET_METHOD, POST_METHOD},
		AllowHeaders:     allowHeaders,
		AllowCredentials: true,
	}))

	sslRedirect, err := strconv.ParseBool(common.GetenvWithDefault("SSL_REDIRECT", "false"))
	if nil != err {
		sslRedirect = false
		log.Print(err)
	}
	log.Printf("Setting SSL redirect to %t", sslRedirect)

	martini_.Use(secure.Secure(secure.Options{
		SSLRedirect: sslRedirect,
	}))

	//Backers information
	martini_.Get(CAMPAIGN_URL, getCampaignHandler, errorHandler)
	martini_.Head(CAMPAIGN_URL, getCampaignHandler, errorHandler)

	//Perks information
	martini_.Get(PERKS_URL, getPerkHandler, errorHandler)
	martini_.Head(PERKS_URL, getPerkHandler, errorHandler)

	//Payments information
	martini_.Get(PAYMENTS_URL, getPaymentHandler, errorHandler)
	martini_.Head(PAYMENTS_URL, getPaymentHandler, errorHandler)

	//Accept payments
	martini_.Post(PAYMENTS_URL, binding.Form(Payment{}), errorHandler, makePaymentHandler)

	martini_.NotFound(notFoundHandler)
	martini_.Run()
}

func main() {
	dbUrl := os.Getenv("DATABASE_URL")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbHost := common.GetenvWithDefault("DB_HOST", "localhost")
	dbPort := common.GetenvWithDefault("DB_PORT", "5432")
	dbMaxOpenConnsStr := common.GetenvWithDefault("DB_MAX_OPEN_CONNS", "10")
	dbMaxIdleConnsStr := common.GetenvWithDefault("DB_MAX_IDLE_CONNS", "0")

	var err error

	dbMaxOpenConns, err := strconv.Atoi(dbMaxOpenConnsStr)
	if nil != err {
		dbMaxOpenConns = 10
		log.Printf("Error setting database maximum open connections from value: %s. Default to %d", dbMaxOpenConnsStr, dbMaxOpenConns)
		log.Print(err)
	}

	dbMaxIdleConns, err := strconv.Atoi(dbMaxIdleConnsStr)
	if nil != err {
		dbMaxIdleConns = 0
		log.Printf("Error setting database maximum idle connections from value: %s. Default to %d", dbMaxIdleConnsStr, dbMaxIdleConns)
		log.Print(err)
	}

	dbCredentials := common.DatabaseCredentials{common.DB_DRIVER, dbUrl, dbUser, dbPassword, dbName, dbHost, dbPort, dbMaxOpenConns, dbMaxIdleConns}
	if !dbCredentials.IsValid() {
		log.Fatalf("Database credentials NOT set correctly. %#v", dbCredentials)
	}

	//Database connection
	log.Print("Enabling database connectivity")

	db = dbCredentials.GetDatabase()
	defer db.Close()

	//Get configurable string size limits
	stringSizeLimitStr := common.GetenvWithDefault("STRING_SIZE_LIMIT", "500")
	stringSizeLimit, err = strconv.Atoi(stringSizeLimitStr)
	if nil != err {
		stringSizeLimit = 500
		log.Printf("Error setting string size limit from value: %s. Default to %d", stringSizeLimitStr, stringSizeLimit)
		log.Print(err)
	}

	//E-mail regular expression
	log.Print("Compiling e-mail regular expression")
	emailRegex, err = regexp.Compile(EMAIL_REGEX)
	if nil != err {
		log.Print(err)
		log.Fatalf("E-mail regex compilation failed for %s", EMAIL_REGEX)
	}

	//Robot detection field
	botDetectionFieldLocationStr := common.GetenvWithDefault("BOTDETECT_FIELDLOCATION", "body")
	botDetectionFieldName := common.GetenvWithDefault("BOTDETECT_FIELDNAME", "spambot")
	botDetectionFieldValue := common.GetenvWithDefault("BOTDETECT_FIELDVALUE", "")
	botDetectionMustMatchStr := common.GetenvWithDefault("BOTDETECT_MUSTMATCH", "true")
	botDetectionPlayCoyStr := common.GetenvWithDefault("BOTDETECT_PLAYCOY", "true")

	var botDetectionFieldLocation RequestLocation

	switch botDetectionFieldLocationStr {
	case "header":
		botDetectionFieldLocation = Header
		break
	case "body":
		botDetectionFieldLocation = Body
		break
	default:
		botDetectionFieldLocation = Body
		log.Printf("Error with int input for field %s with value %s.  Defaulting to Body.", "BOTDETECT_FIELDLOCATION", botDetectionFieldLocationStr)
		break
	}

	botDetectionMustMatch, err := strconv.ParseBool(botDetectionMustMatchStr)
	if nil != err {
		botDetectionMustMatch = true
		log.Printf("Error converting boolean input for field %s with value %s. Defaulting to true.", "BOTDETECT_MUSTMATCH", botDetectionMustMatchStr)
		log.Print(err)
	}

	botDetectionPlayCoy, err := strconv.ParseBool(botDetectionPlayCoyStr)
	if nil != err {
		botDetectionPlayCoy = true
		log.Printf("Error converting boolean input for field %s with value %s. Defaulting to true.", "BOTDETECT_PLAYCOY", botDetectionPlayCoyStr)
		log.Print(err)
	}

	botDetection = BotDetection{botDetectionFieldLocation, botDetectionFieldName, botDetectionFieldValue, botDetectionMustMatch, botDetectionPlayCoy}

	log.Printf("Creating robot detection with %#v", botDetection)

	//Asynchronous database writes
	asyncRequest, err = strconv.ParseBool(common.GetenvWithDefault("ASYNC_REQUEST", "false"))
	if nil != err {
		asyncRequest = false
		running = false
		log.Printf("Error converting input for field ASYNC_REQUEST. Defaulting to false.")
		log.Print(err)
	}

	asyncRequestSizeStr := common.GetenvWithDefault("ASYNC_REQUEST_SIZE", "100000")
	asyncRequestSize, err := strconv.Atoi(asyncRequestSizeStr)
	if nil != err {
		asyncRequestSize = 100000
		log.Printf("Error converting input for field ASYNC_REQUEST_SIZE. Defaulting to 100000.")
		log.Print(err)
	}

	asyncProcessIntervalStr := common.GetenvWithDefault("ASYNC_PROCESS_INTERVAL", "5")
	asyncProcessInterval, err := strconv.Atoi(asyncProcessIntervalStr)
	if nil != err {
		asyncProcessInterval = 5
		log.Printf("Error converting input for field ASYNC_PROCESS_INTERVAL. Defaulting to 5.")
		log.Print(err)
	}

	if asyncRequest {
		waitGroup.Add(1)
		running = true
		payments = make(chan Payment, asyncRequestSize)
		go batchAddPayment(db, time.Duration(asyncProcessInterval), dbMaxOpenConns)
		log.Printf("Asynchronous requests enabled. Request queue size set to %d", asyncRequestSize)
		log.Printf("Asynchronous process interval is %d seconds", asyncProcessInterval)
	}

	//Signal handler
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt)
	signal.Notify(signals, syscall.SIGTERM)
	go func() {
		<-signals
		log.Print("Shutting down...")
		running = false
		waitGroup.Wait()
		os.Exit(0)
	}()

	//HTTP server
	host := common.GetenvWithDefault("HOST", "")
	port := common.GetenvWithDefault("PORT", "3000")
	mode := common.GetenvWithDefault("MARTINI_ENV", "development")

	log.Printf("Running HTTP server on %s:%s in mode %s", host, port, mode)
	runHttpServer()
}
