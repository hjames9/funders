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
	"sync"
	"time"
)

const (
	ADD_PLEDGE_QUERY = "INSERT INTO funders.pledges(id, campaign_id, perk_id, contact_email, phone_number, contact_opt_in, amount, currency, advertise, advertise_name, created_at, updated_at) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id"
	PLEDGES_URL      = "/pledges"
)

type Pledge struct {
	Id            string
	CampaignId    int64 `form:"campaignId" binding:"required"`
	Campaign      *Campaign
	PerkId        int64 `form:"perkId" binding:"required"`
	Perk          *Perk
	ContactEmail  string `form:"contactEmail"`
	PhoneNumber   string `form:"phoneNumber"`
	ContactOptIn  bool   `form:"contactOptIn"`
	Amount        float64
	Currency      string
	Advertise     bool   `form:"advertise"`
	AdvertiseName string `form:"advertiseName"`
}

func (pledge *Pledge) MarshalJSON() ([]byte, error) {
	type MyPledge Pledge
	return json.Marshal(&struct {
		Id         string    `json:"id"`
		CampaignId int64     `json:"campaignId"`
		Campaign   *Campaign `json:"campaign"`
		PerkId     int64     `json:"perkId"`
		Perk       *Perk     `json:"perk"`
	}{
		Id:         pledge.Id,
		CampaignId: pledge.CampaignId,
		Campaign:   pledge.Campaign,
		PerkId:     pledge.PerkId,
		Perk:       pledge.Perk,
	})
}

//Asynchronous pledges
var asyncPledgeRequest bool

func (pledge *Pledge) Validate(errors binding.Errors, req *http.Request) binding.Errors {
	errors = validateSizeLimit(pledge.ContactEmail, "contactEmail", stringSizeLimit, errors)
	errors = validateSizeLimit(pledge.PhoneNumber, "phoneNumber", stringSizeLimit, errors)
	errors = validateSizeLimit(pledge.AdvertiseName, "advertiseName", stringSizeLimit, errors)

	if len(errors) == 0 {
		if len(pledge.ContactEmail) == 0 && len(pledge.PhoneNumber) == 0 {
			errors = addError(errors, []string{"contactEmail", "phoneNumber"}, binding.RequiredError, "Either contact email or phone number is required")
		}

		if len(pledge.ContactEmail) > 0 && !emailRegex.MatchString(pledge.ContactEmail) {
			message := fmt.Sprintf("Invalid email \"%s\" format specified", pledge.ContactEmail)
			errors = addError(errors, []string{"contactEmail"}, binding.TypeError, message)
		}

		if pledge.Advertise && len(pledge.AdvertiseName) == 0 {
			errors = addError(errors, []string{"advertise", "advertiseName"}, binding.TypeError, "Allowing advertisement without providing name")
		}

		perk, exists := perks.GetPerk(pledge.PerkId)
		if exists {
			if !perk.IsAvailable() {
				message := fmt.Sprintf("Perk is not available. (%d/%d) claimed or pledged", perk.NumClaimed+perk.NumPledged, perk.Available)
				errors = addError(errors, []string{"perkId"}, binding.TypeError, message)
			} else {
				pledge.Amount = perk.Price
				pledge.Currency = perk.Currency
				pledge.Perk = perk
			}
		} else {
			message := fmt.Sprintf("Perk not found with id: %d for campaign: %d", pledge.PerkId, pledge.CampaignId)
			errors = addError(errors, []string{"perkId"}, binding.TypeError, message)
		}

		campaign, exists := campaigns.GetCampaignById(pledge.CampaignId)
		if exists {
			if campaign.HasEnded() {
				message := fmt.Sprintf("Campaign %s with id: %d has expired on %s", campaign.Name, pledge.CampaignId, campaign.EndDate)
				errors = addError(errors, []string{"campaignId"}, binding.TypeError, message)
			} else if !campaign.HasStarted() {
				message := fmt.Sprintf("Campaign %s with id: %d will start on %s", campaign.Name, pledge.CampaignId, campaign.StartDate)
				errors = addError(errors, []string{"campaignId"}, binding.TypeError, message)
			} else {
				pledge.Campaign = campaign
			}
		} else {
			message := fmt.Sprintf("Campaign not found with id: %d", pledge.CampaignId)
			errors = addError(errors, []string{"campaignId"}, binding.TypeError, message)
		}

		if botDetection.IsBot(req) {
			message := "Go away spambot! We've alerted the authorities"
			errors = addError(errors, []string{"spambot"}, common.BOT_ERROR, message)
		}
	}

	return errors
}

//Background pledge threads
var pledgeBatchProcessor *common.BatchProcessor

func processPledge(pledge *Pledge) error {
	err := addPledge(pledge, nil)
	if nil == err {
		makePledge(pledge, nil)
	}
	return err
}

func processBatchPledge(pledgeBatch []interface{}, waitGroup *sync.WaitGroup) {
	log.Printf("Starting batch processing of %d pledges", len(pledgeBatch))

	defer waitGroup.Done()

	transaction, err := db.Begin()
	if nil != err {
		log.Print("Error creating transaction")
		log.Print(err)
	}

	defer transaction.Rollback()
	statement, err := transaction.Prepare(ADD_PLEDGE_QUERY)
	if nil != err {
		log.Print("Error preparing SQL statement")
		log.Print(err)
	}

	defer statement.Close()

	counter := 0
	for _, pledgeInterface := range pledgeBatch {
		pledge := pledgeInterface.(*Pledge)
		err = addPledge(pledge, statement)
		if nil != err {
			log.Printf("Error processing pledge %#v", pledge)
			log.Print(err)
			continue
		}

		counter++
		waitGroup.Add(1)
		go makePledge(pledge, waitGroup)
	}

	err = transaction.Commit()
	if nil != err {
		log.Print("Error committing transaction")
		log.Print(err)
	} else {
		log.Printf("Processed %d pledges", counter)
	}
}

func addPledge(pledge *Pledge, statement *sql.Stmt) error {
	var err error

	contactEmail := common.CreateSqlString(pledge.ContactEmail)
	phoneNumber := common.CreateSqlString(pledge.PhoneNumber)
	advertiseName := common.CreateSqlString(pledge.AdvertiseName)

	if nil == statement {
		err = db.QueryRow(ADD_PLEDGE_QUERY, pledge.Id, pledge.CampaignId, pledge.PerkId, contactEmail, phoneNumber, pledge.ContactOptIn, pledge.Amount, pledge.Currency, pledge.Advertise, advertiseName, time.Now(), time.Now()).Scan(&pledge.Id)
	} else {
		err = statement.QueryRow(pledge.Id, pledge.CampaignId, pledge.PerkId, contactEmail, phoneNumber, pledge.ContactOptIn, pledge.Amount, pledge.Currency, pledge.Advertise, advertiseName, time.Now(), time.Now()).Scan(&pledge.Id)
	}
	if nil == err {
		log.Printf("New pledge id = %s", pledge.Id)
	}

	return err
}

func makePledge(pledge *Pledge, waitGroup *sync.WaitGroup) {
	if nil != waitGroup {
		defer waitGroup.Done()
	}

	perk, exists := perks.GetPerk(pledge.PerkId)
	if exists {
		perk.IncrementNumPledged(1)
	} else {
		log.Printf("Perk %d not found for campaign %d", pledge.PerkId, pledge.CampaignId)
	}

	campaign, exists := campaigns.GetCampaignById(pledge.CampaignId)
	if exists {
		campaign.IncrementNumPledged(pledge.Amount)
		campaign.IncrementNumPledgers(1)
		advertisements.AddAdvertisementFromPledge(campaign.Name, pledge)
	} else {
		log.Printf("Campaign %d not found", pledge.CampaignId)
	}
}

func makePledgeHandler(res http.ResponseWriter, req *http.Request, pledge Pledge) (int, string) {
	pledge.Id = uuid.NewV4().String()

	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	res.Header().Set(LOCATION_HEADER, fmt.Sprintf("%s?id=%s", PLEDGES_URL, pledge.Id))

	log.Printf("Received new pledge: %#v", pledge)

	req.Close = true
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	var response common.Response

	if asyncPledgeRequest && nil != pledgeBatchProcessor && pledgeBatchProcessor.Running {
		pledgeBatchProcessor.AddEvent(&pledge)
		responseStr := "Successfully scheduled pledge"
		response = common.Response{Code: http.StatusAccepted, Message: responseStr, Id: pledge.Id}
		log.Print(responseStr)
	} else if !asyncPledgeRequest {
		err := processPledge(&pledge)
		if nil != err {
			response = common.Response{Code: http.StatusInternalServerError, Message: err.Error(), Id: pledge.Id}
			log.Print(err)
		} else {
			jsonStr, _ := json.Marshal(&pledge)
			return http.StatusCreated, string(jsonStr)
		}
	} else if asyncPledgeRequest && (nil == pledgeBatchProcessor || !pledgeBatchProcessor.Running) {
		responseStr := "Could not add pledge due to server maintenance"
		response = common.Response{Code: http.StatusServiceUnavailable, Message: responseStr, Id: pledge.Id}
		log.Print(responseStr)
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}
