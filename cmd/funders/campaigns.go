package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/hjames9/funders"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	GET_ALL_CAMPAIGNS_QUERY = "SELECT id, name, description, goal, num_raised, num_backers, num_pledged, num_pledgers, start_date, end_date, flexible FROM funders.campaign_backers WHERE active = TRUE"
	GET_CAMPAIGN_QUERY      = "SELECT id, name, description, goal, num_raised, num_backers, num_pledged, num_pledgers, start_date, end_date, flexible FROM funders.campaign_backers WHERE active = TRUE AND name = $1"
	CAMPAIGN_URL            = "/campaigns"
)

type Campaign common.Campaign

func (campaign *Campaign) IncrementNumRaised(amount float64) float64 {
	campaign.Lock.Lock()
	defer campaign.Lock.Unlock()
	campaign.NumRaised += amount
	return campaign.NumRaised
}

func (campaign *Campaign) IncrementNumBackers(amount int64) int64 {
	campaign.Lock.Lock()
	defer campaign.Lock.Unlock()
	campaign.NumBackers += amount
	return campaign.NumBackers
}

func (campaign *Campaign) IncrementNumPledged(amount float64) float64 {
	campaign.Lock.Lock()
	defer campaign.Lock.Unlock()
	campaign.NumPledged += amount
	return campaign.NumPledged
}

func (campaign *Campaign) IncrementNumPledgers(amount int64) int64 {
	campaign.Lock.Lock()
	defer campaign.Lock.Unlock()
	campaign.NumPledgers += amount
	return campaign.NumPledgers
}

func (campaign *Campaign) HasStarted() bool {
	return time.Now().After(campaign.StartDate)
}

func (campaign *Campaign) HasEnded() bool {
	return time.Now().After(campaign.EndDate)
}

func (campaign *Campaign) MarshalJSON() ([]byte, error) {
	campaign.Lock.RLock()
	numRaised := campaign.NumRaised
	numBackers := campaign.NumBackers
	numPledged := campaign.NumPledged
	numPledgers := campaign.NumPledgers
	campaign.Lock.RUnlock()

	type MyCampaign Campaign
	return json.Marshal(&struct {
		NumRaised   float64 `json:"numRaised"`
		NumBackers  int64   `json:"numBackers"`
		NumPledged  float64 `json:"numPledged"`
		NumPledgers int64   `json:"numPledgers"`
		*MyCampaign
	}{
		NumRaised:   numRaised,
		NumBackers:  numBackers,
		NumPledged:  numPledged,
		NumPledgers: numPledgers,
		MyCampaign:  (*MyCampaign)(campaign),
	})
}

type Campaigns struct {
	lock       sync.RWMutex
	nameValues map[string]*Campaign
	idValues   map[int64]*Campaign
}

func NewCampaigns() *Campaigns {
	campaigns := new(Campaigns)
	campaigns.nameValues = make(map[string]*Campaign)
	campaigns.idValues = make(map[int64]*Campaign)
	return campaigns
}

func (cm *Campaigns) AddOrReplaceCampaign(campaign *Campaign) *Campaign {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	cm.nameValues[campaign.Name] = campaign
	cm.idValues[campaign.Id] = campaign
	return campaign
}

func (cm *Campaigns) AddOrReplaceCampaigns(campaigns []*Campaign) {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	for _, campaign := range campaigns {
		cm.nameValues[campaign.Name] = campaign
		cm.idValues[campaign.Id] = campaign
	}
}

func (cm *Campaigns) GetCampaign(name string) (*Campaign, bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	val, exists := cm.nameValues[name]
	return val, exists
}

func (cm *Campaigns) GetCampaignById(id int64) (*Campaign, bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	val, exists := cm.idValues[id]
	return val, exists
}

var campaigns = NewCampaigns()

func getCampaignsFromDb() ([]*Campaign, error) {
	rows, err := db.Query(GET_ALL_CAMPAIGNS_QUERY)
	if nil != err {
		return nil, err
	}

	defer rows.Close()

	var campaigns []*Campaign
	for rows.Next() {
		var campaign Campaign
		err = rows.Scan(&campaign.Id, &campaign.Name, &campaign.Description, &campaign.Goal, &campaign.NumRaised, &campaign.NumBackers, &campaign.NumPledged, &campaign.NumPledgers, &campaign.StartDate, &campaign.EndDate, &campaign.Flexible)
		if nil == err {
			campaigns = append(campaigns, &campaign)
		} else {
			break
		}
	}

	if nil == err {
		err = rows.Err()
	}

	return campaigns, err
}

func getCampaignFromDb(name string) (Campaign, error) {
	var campaign Campaign
	err := db.QueryRow(GET_CAMPAIGN_QUERY, name).Scan(&campaign.Id, &campaign.Name, &campaign.Description, &campaign.Goal, &campaign.NumRaised, &campaign.NumBackers, &campaign.NumPledged, &campaign.NumPledgers, &campaign.StartDate, &campaign.EndDate, &campaign.Flexible)
	return campaign, err
}

func getCampaign(name string) (*Campaign, error) {
	var err error
	campaign, exists := campaigns.GetCampaign(name)
	if !exists {
		var campaignDb Campaign
		campaignDb, err = getCampaignFromDb(name)
		if nil == err {
			campaign = campaigns.AddOrReplaceCampaign(&campaignDb)
			log.Print("Retrieved campaign from database")
		} else {
			log.Print("Campaign not found in database")
		}
	} else {
		log.Print("Retrieved campaign from cache")
	}

	return campaign, err
}

func getCampaignHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	req.Close = true

	var response common.Response
	campaignName := strings.TrimSpace(req.URL.Query().Get("name"))

	if len(campaignName) == 0 {
		responseStr := "Campaign name parameter required"
		response = common.Response{Code: http.StatusBadRequest, Message: responseStr}
	} else {
		campaign, err := getCampaign(campaignName)

		if sql.ErrNoRows == err {
			responseStr := fmt.Sprintf("%s not found", campaignName)
			response = common.Response{Code: http.StatusNotFound, Message: responseStr}
			log.Print(responseStr)
		} else if nil != err {
			responseStr := "Could not get campaign due to server error"
			response = common.Response{Code: http.StatusInternalServerError, Message: responseStr}
			log.Print(err)
		} else {
			jsonStr, _ := json.Marshal(campaign)
			return http.StatusOK, string(jsonStr)
		}
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}
