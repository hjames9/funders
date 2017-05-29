package main

import (
	"bitbucket.org/padium/funders"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	_ "github.com/lib/pq"
	"github.com/logpacker/PayPal-Go-SDK"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/cors"
	"github.com/martini-contrib/gzip"
	"github.com/martini-contrib/secure"
	"github.com/satori/go.uuid"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	CONTENT_TYPE_HEADER = "Content-Type"
	LOCATION_HEADER     = "Location"
	ORIGIN_HEADER       = "Origin"
	JSON_CONTENT_TYPE   = "application/json"
	XML_CONTENT_TYPE    = "application/xml"
	TEXT_CONTENT_TYPE   = "text/plain"
	GET_METHOD          = "GET"
	HEAD_METHOD         = "HEAD"
	POST_METHOD         = "POST"
	PUT_METHOD          = "PUT"
	PATCH_METHOD        = "PATCH"
	ROBOTS_TXT_URL      = "/robots.txt"
	SITEMAP_XML_URL     = "/sitemap.xml"
	FAVICON_ICO_URL     = "/favicon.ico"
)

var db *sql.DB
var stringSizeLimit int
var botDetection common.BotDetection
var gzipResponse bool
var gzipCompressionLevel int
var robotsTxtResponse bool
var sitemapXmlResponse bool
var faviconIcoResponse bool

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
		var response common.Response

		if errors.Has(binding.RequiredError) {
			res.WriteHeader(http.StatusBadRequest)
			responseStr := fmt.Sprintf("Missing required field(s): %s", fieldsMsg)
			response = common.Response{Code: http.StatusBadRequest, Message: responseStr}
		} else if errors.Has(binding.ContentTypeError) {
			res.WriteHeader(http.StatusUnsupportedMediaType)
			response = common.Response{Code: http.StatusUnsupportedMediaType, Message: "Invalid content type"}
		} else if errors.Has(binding.DeserializationError) {
			res.WriteHeader(http.StatusBadRequest)
			response = common.Response{Code: http.StatusBadRequest, Message: "Deserialization error"}
		} else if errors.Has(binding.TypeError) {
			res.WriteHeader(http.StatusBadRequest)
			response = common.Response{Code: http.StatusBadRequest, Message: errors[0].Error()}
		} else if errors.Has(common.BOT_ERROR) {
			if botDetection.PlayCoy && !paymentBatchProcessor.Running {
				res.WriteHeader(http.StatusCreated)
				response = common.Response{Code: http.StatusCreated, Message: "Successfully added payment", Id: uuid.NewV4().String()}
				log.Printf("Robot detected: %s. Playing coy.", errors[0].Error())
			} else if botDetection.PlayCoy && paymentBatchProcessor.Running {
				res.WriteHeader(http.StatusAccepted)
				response = common.Response{Code: http.StatusAccepted, Message: "Successfully scheduled payment"}
				log.Printf("Robot detected: %s. Playing coy.", errors[0].Error())
			} else {
				res.WriteHeader(http.StatusBadRequest)
				response = common.Response{Code: http.StatusBadRequest, Message: errors[0].Error()}
				log.Printf("Robot detected: %s. Rejecting message.", errors[0].Error())
			}
		} else {
			res.WriteHeader(http.StatusBadRequest)
			response = common.Response{Code: http.StatusBadRequest, Message: "Unknown error"}
		}

		log.Print(response.Message)
		jsonStr, err := json.Marshal(response)
		if nil != err {
			log.Print(err)
		} else {
			res.Write(jsonStr)
		}
	}
}

func notFoundHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	req.Close = true
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	responseStr := fmt.Sprintf("URL Not Found %s", req.URL)
	response := common.Response{Code: http.StatusNotFound, Message: responseStr}
	log.Print(responseStr)
	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}

func runHttpServer() {
	martini_ := martini.Classic()

	allowHeaders := []string{ORIGIN_HEADER}
	if botDetection.FieldLocation == common.Header {
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

	//GZIP responses
	if gzipResponse {
		martini_.Use(gzip.All(gzip.Options{CompressionLevel: gzipCompressionLevel}))
	}

	martini_.Use(cors.Allow(&cors.Options{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{GET_METHOD, HEAD_METHOD, POST_METHOD, PUT_METHOD, PATCH_METHOD},
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
		SSLRedirect:     sslRedirect,
		SSLProxyHeaders: map[string]string{"X-Forwarded-Proto": "https"},
	}))

	//Campaigns information
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

	//Update payments
	martini_.Put(PAYMENTS_URL, binding.Form(UpdatePayment{}), errorHandler, updatePaymentHandler)
	martini_.Patch(PAYMENTS_URL, binding.Form(UpdatePayment{}), errorHandler, updatePaymentHandler)

	//Accept pledges
	martini_.Post(PLEDGES_URL, binding.Form(Pledge{}), errorHandler, makePledgeHandler)

	//Advertise payments
	martini_.Get(ADVERTISEMENTS_URL, getAdvertisementHandler, errorHandler)
	martini_.Head(ADVERTISEMENTS_URL, getAdvertisementHandler, errorHandler)

	//robots.txt
	if robotsTxtResponse {
		getRobotsTxt := func(res http.ResponseWriter, req *http.Request) (int, string) {
			res.Header().Set(CONTENT_TYPE_HEADER, TEXT_CONTENT_TYPE)
			var robotsTxt common.RobotsTxt
			robotsTxt.AddRecord(common.RobotsRecord{[]string{"*"}, []string{"/"}})
			return http.StatusOK, robotsTxt.String()
		}
		martini_.Get(ROBOTS_TXT_URL, getRobotsTxt, errorHandler)
		martini_.Head(ROBOTS_TXT_URL, getRobotsTxt, errorHandler)
	}

	//sitemap.xml
	if sitemapXmlResponse {
		getSitemapXml := func(res http.ResponseWriter, req *http.Request) (int, string) {
			res.Header().Set(CONTENT_TYPE_HEADER, XML_CONTENT_TYPE)

			hostname := fmt.Sprintf("%s://%s", common.GetScheme(req), req.Host)
			var urlSet common.UrlSet

			campaigns.lock.RLock()
			defer campaigns.lock.RUnlock()
			for key, _ := range campaigns.nameValues {
				url := fmt.Sprintf("%s%s?name=%s", hostname, CAMPAIGN_URL, key)
				urlSet.AddUrl(common.Url{Location: url, LastModification: time.Now(), ChangeFrequency: common.Always, Priority: 1.0})
			}

			perks.lock.RLock()
			defer perks.lock.RUnlock()
			for key, _ := range perks.nameValues {
				url := fmt.Sprintf("%s%s?campaign_name=%s", hostname, PERKS_URL, key)
				urlSet.AddUrl(common.Url{Location: url, LastModification: time.Now(), ChangeFrequency: common.Always, Priority: 0.8})
			}

			advertisements.lock.RLock()
			defer advertisements.lock.RUnlock()
			for key, _ := range advertisements.nameValues {
				url := fmt.Sprintf("%s%s?campaign_name=%s", hostname, ADVERTISEMENTS_URL, key)
				urlSet.AddUrl(common.Url{Location: url, LastModification: time.Now(), ChangeFrequency: common.Always, Priority: 0.3})
			}

			return http.StatusOK, urlSet.String()
		}
		martini_.Get(SITEMAP_XML_URL, getSitemapXml, errorHandler)
		martini_.Head(SITEMAP_XML_URL, getSitemapXml, errorHandler)
	}

	//favicon.ico
	if faviconIcoResponse {
		getFaviconIco := func(res http.ResponseWriter, req *http.Request) (int, string) {
			res.Header().Set(CONTENT_TYPE_HEADER, common.ICO_CONTENT_TYPE)
			var favicon common.FaviconIco
			favicon.CreateImage(0xA384C1FF, 16, 16)
			return http.StatusOK, string(favicon.GetImageData())
		}
		martini_.Get(FAVICON_ICO_URL, getFaviconIco, errorHandler)
		martini_.Head(FAVICON_ICO_URL, getFaviconIco, errorHandler)
	}

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

	//Get access key for stripe
	stripeKey = os.Getenv("STRIPE_KEY")
	if len(stripeKey) > 0 {
		log.Printf("Stripe key is set to: %s", stripeKey)
	} else {
		log.Fatal("Stripe key is NOT set")
	}

	//Get client and secret ids for paypal
	paypalClientId := os.Getenv("PAYPAL_CLIENT_ID")
	paypalSecretId := os.Getenv("PAYPAL_SECRET_ID")
	if len(paypalClientId) > 0 && len(paypalSecretId) > 0 {
		log.Printf("Paypal client id is %s, secret id is %s", paypalClientId, paypalSecretId)
	} else {
		log.Fatal("Both PAYPAL_CLIENT_ID and PAYPAL_SECRET_ID need to be set")
	}

	paypalClient, err = paypalsdk.NewClient(paypalClientId, paypalSecretId, paypalsdk.APIBaseSandBox)
	if nil != err {
		log.Print(err)
		log.Fatal("Error creating paypal client")
	} else {
		accessToken, err := paypalClient.GetAccessToken()
		if nil == err {
			log.Printf("Successfully created paypal client with access token %s", accessToken)
		} else {
			log.Print(err)
			log.Fatal("Error creating paypal client with access token")
		}
	}

	//E-mail regular expression
	log.Print("Compiling e-mail regular expression")
	emailRegex, err = regexp.Compile(EMAIL_REGEX)
	if nil != err {
		log.Print(err)
		log.Fatalf("E-mail regex compilation failed for %s", EMAIL_REGEX)
	} else {
		log.Print("E-mail regex compilation succeeded")
	}

	//UUID regular expression
	log.Print("Compiling uuid regular expression")
	uuidRegex, err = regexp.Compile(UUID_REGEX)
	if nil != err {
		log.Print(err)
		log.Fatalf("UUID regex compilation failed for %s", UUID_REGEX)
	} else {
		log.Print("UUID regex compilation succeeded")
	}

	//Allowable account types
	accountTypesStr := getAccountTypes()
	if len(accountTypesStr) > 0 {
		accountTypes = make(map[string]bool)

		accountTypesArr := strings.Split(accountTypesStr, ",")
		for _, accountType := range accountTypesArr {
			accountTypes[accountType] = true
		}

		log.Printf("Allowable account types: %s", accountTypesStr)
	} else {
		log.Fatal("Unable to retrieve account types from database")
	}

	//Allowable payment statuses
	paymentStatusesStr := getPaymentStatuses()
	if len(paymentStatusesStr) > 0 {
		paymentStatuses = make(map[string]bool)

		paymentStatusesArr := strings.Split(paymentStatusesStr, ",")
		for _, status := range paymentStatusesArr {
			paymentStatuses[status] = true
		}

		log.Printf("Allowable payment statuses: %s", paymentStatusesStr)
	} else {
		log.Fatal("Unable to retrieve payment statuses from database")
	}

	//Robot detection field
	botDetectionFieldLocationStr := common.GetenvWithDefault("BOTDETECT_FIELDLOCATION", "body")
	botDetectionFieldName := common.GetenvWithDefault("BOTDETECT_FIELDNAME", "spambot")
	botDetectionFieldValue := common.GetenvWithDefault("BOTDETECT_FIELDVALUE", "")
	botDetectionMustMatchStr := common.GetenvWithDefault("BOTDETECT_MUSTMATCH", "true")
	botDetectionPlayCoyStr := common.GetenvWithDefault("BOTDETECT_PLAYCOY", "true")

	var botDetectionFieldLocation common.RequestLocation

	switch botDetectionFieldLocationStr {
	case "header":
		botDetectionFieldLocation = common.Header
		break
	case "body":
		botDetectionFieldLocation = common.Body
		break
	default:
		botDetectionFieldLocation = common.Body
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

	botDetection = common.BotDetection{botDetectionFieldLocation, botDetectionFieldName, botDetectionFieldValue, botDetectionMustMatch, botDetectionPlayCoy}

	log.Printf("Creating robot detection with %#v", botDetection)

	//GZIP response compression
	gzipCompressionLevelStr := common.GetenvWithDefault("GZIP_COMPRESSION_LEVEL", "6")
	gzipCompressionLevel, err = strconv.Atoi(gzipCompressionLevelStr)
	if nil != err {
		gzipCompressionLevel = 6
		log.Printf("Error setting gzip compression level from value: %s. Default to %d", gzipCompressionLevelStr, gzipCompressionLevel)
		log.Print(err)
	} else if gzipCompressionLevel < 1 || gzipCompressionLevel > 9 {
		gzipCompressionLevel = 6
		log.Printf("Error setting gzip compression level from value: %d. Default to %d", gzipCompressionLevelStr, gzipCompressionLevel)
	}

	gzipResponseStr := common.GetenvWithDefault("GZIP_RESPONSE", "true")
	gzipResponse, err = strconv.ParseBool(gzipResponseStr)
	if nil != err {
		gzipResponse = true
		log.Printf("Error converting boolean input for field %s with value %s. Defaulting to true.", "GZIP_RESPONSE", gzipResponseStr)
		log.Print(err)
	}
	if gzipResponse {
		log.Printf("Gzip response encoding enabled with level %d", gzipCompressionLevel)
	} else {
		log.Print("Gzip response encoding disabled")
	}

	//Asynchronous payment processing
	asyncRequestSizeStr := common.GetenvWithDefault("ASYNC_REQUEST_SIZE", "100000")
	asyncRequestSize, err := strconv.Atoi(asyncRequestSizeStr)
	if nil != err {
		asyncRequestSize = 100000
		log.Printf("Error converting input for field ASYNC_REQUEST_SIZE. Defaulting to 100000.")
		log.Print(err)
	}
	log.Printf("Asynchronous request queue size set to %d", asyncRequestSize)

	asyncProcessIntervalStr := common.GetenvWithDefault("ASYNC_PROCESS_INTERVAL", "5")
	asyncProcessInterval, err := strconv.Atoi(asyncProcessIntervalStr)
	if nil != err {
		asyncProcessInterval = 5
		log.Printf("Error converting input for field ASYNC_PROCESS_INTERVAL. Defaulting to 5.")
		log.Print(err)
	}
	log.Printf("Asynchronous process interval is %d seconds", asyncProcessInterval)

	//Asynchronous payment request
	asyncPaymentRequestStr := common.GetenvWithDefault("ASYNC_PAYMENT_REQUEST", "true")
	asyncPaymentRequest, err = strconv.ParseBool(asyncPaymentRequestStr)
	if nil != err {
		asyncPaymentRequest = true
		log.Printf("Error converting boolean input for field %s with value %s. Defaulting to true.", "ASYNC_PAYMENT_REQUEST", asyncPaymentRequestStr)
		log.Print(err)
	} else if asyncPaymentRequest {
		//Make payment processor
		paymentBatchProcessor = common.NewBatchProcessor(processBatchPayment, asyncRequestSize, asyncProcessInterval, dbMaxOpenConns)
		paymentBatchProcessor.Start()
		log.Print("Asynchronous payment requests enabled")
	} else {
		log.Print("Synchronous payment requests enabled")
	}

	//Asynchronous update payment request
	asyncUpdatePaymentRequestStr := common.GetenvWithDefault("ASYNC_UPDATE_PAYMENT_REQUEST", "true")
	asyncUpdatePaymentRequest, err = strconv.ParseBool(asyncUpdatePaymentRequestStr)
	if nil != err {
		asyncUpdatePaymentRequest = true
		log.Printf("Error converting boolean input for field %s with value %s. Defaulting to true.", "ASYNC_UPDATE_PAYMENT_REQUEST", asyncUpdatePaymentRequestStr)
		log.Print(err)
	} else if asyncUpdatePaymentRequest {
		//Update payment processor
		updatePaymentBatchProcessor = common.NewBatchProcessor(processBatchUpdatePayment, asyncRequestSize, asyncProcessInterval, dbMaxOpenConns)
		updatePaymentBatchProcessor.Start()
		log.Print("Asynchronous update payment requests enabled")
	} else {
		log.Print("Synchronous update payment requests enabled")
	}

	//Asynchronous payment request
	asyncPledgeRequestStr := common.GetenvWithDefault("ASYNC_PLEDGE_REQUEST", "true")
	asyncPledgeRequest, err = strconv.ParseBool(asyncPledgeRequestStr)
	if nil != err {
		asyncPledgeRequest = true
		log.Printf("Error converting boolean input for field %s with value %s. Defaulting to true.", "ASYNC_PLEDGE_REQUEST", asyncPledgeRequestStr)
		log.Print(err)
	} else if asyncPledgeRequest {
		//Make pledge processor
		pledgeBatchProcessor = common.NewBatchProcessor(processBatchPledge, asyncRequestSize, asyncProcessInterval, dbMaxOpenConns)
		pledgeBatchProcessor.Start()
		log.Print("Asynchronous pledge requests enabled")
	} else {
		log.Print("Synchronous pledge requests enabled")
	}

	//Initialize campaigns
	cmps, err := getCampaignsFromDb()
	if nil != err {
		log.Print(err)
		log.Fatal("Could not initialize campaigns")
	} else {
		campaigns.AddOrReplaceCampaigns(cmps)
		log.Printf("Initialized %d campaigns", len(cmps))
	}

	//Initialize perks
	prks, err := getPerksFromDb()
	if nil != err {
		log.Print(err)
		log.Fatal("Could not initialize perks")
	} else {
		perks.AddOrReplacePerks(prks)
		log.Printf("Initialized %d perks", len(prks))
	}

	//Initialize payments
	pys, err := getPaymentsFromDb()
	if nil != err {
		log.Print(err)
		log.Fatal("Could not initialize payments")
	} else {
		paymentsCache.AddOrReplacePayments(pys)
		log.Printf("Initialized %d payments", len(pys))
	}

	//Initialize pledges
	pls, err := getPledgesFromDb()
	if nil != err {
		log.Print(err)
		log.Fatal("Could not initialize pledges")
	} else {
		pledges.AddOrReplacePledges(pls)
		log.Printf("Initialized %d pledges", len(pls))
	}

	//Initialize advertisements
	ads, err := getAdvertisementsFromDb()
	if nil != err {
		log.Print(err)
		log.Fatal("Could not initialize advertisements")
	} else {
		advertisements.AddOrReplaceAdvertisements(ads)
		log.Printf("Initialized %d advertisements", len(ads))
	}

	//robots.txt
	robotsTxtResponseStr := common.GetenvWithDefault("ROBOTS_TXT", "false")
	robotsTxtResponse, err = strconv.ParseBool(robotsTxtResponseStr)
	if nil != err {
		robotsTxtResponse = false
		log.Printf("Error converting boolean input for field %s with value %s. Defaulting to false.", "ROBOTS_TXT", robotsTxtResponseStr)
		log.Print(err)
	}

	if robotsTxtResponse {
		log.Print("robots.txt support enabled")
	} else {
		log.Print("robots.txt support disabled")
	}

	//sitemap.xml
	sitemapXmlResponseStr := common.GetenvWithDefault("SITEMAP_XML", "false")
	sitemapXmlResponse, err = strconv.ParseBool(sitemapXmlResponseStr)
	if nil != err {
		sitemapXmlResponse = false
		log.Printf("Error converting boolean input for field %s with value %s. Defaulting to false.", "SITEMAP_XML", sitemapXmlResponseStr)
		log.Print(err)
	}

	if sitemapXmlResponse {
		log.Print("sitemap.xml support enabled")
	} else {
		log.Print("sitemap.xml support disabled")
	}

	//favicon.ico
	faviconIcoResponseStr := common.GetenvWithDefault("FAVICON_ICO", "false")
	faviconIcoResponse, err = strconv.ParseBool(faviconIcoResponseStr)
	if nil != err {
		faviconIcoResponse = false
		log.Printf("Error converting boolean input for field %s with value %s. Defaulting to false.", "FAVICON_ICO", faviconIcoResponseStr)
		log.Print(err)
	}

	if faviconIcoResponse {
		log.Print("favicon.ico support enabled")
	} else {
		log.Print("favicon.ico support disabled")
	}

	//Signal handler
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt)
	signal.Notify(signals, syscall.SIGTERM)
	go func() {
		<-signals
		log.Print("Shutting down...")

		if nil != paymentBatchProcessor {
			paymentBatchProcessor.Stop()
			log.Print("Payment batch processor shut down")
		}

		if nil != updatePaymentBatchProcessor {
			updatePaymentBatchProcessor.Stop()
			log.Print("Payment update batch processor shut down")
		}

		if nil != pledgeBatchProcessor {
			pledgeBatchProcessor.Stop()
			log.Print("Pledge batch processor shut down")
		}

		os.Exit(0)
	}()

	//HTTP server
	host := common.GetenvWithDefault("HOST", "")
	port := common.GetenvWithDefault("PORT", "3000")
	mode := common.GetenvWithDefault("MARTINI_ENV", "development")

	log.Printf("Running HTTP server on %s:%s in mode %s", host, port, mode)
	runHttpServer()
}
