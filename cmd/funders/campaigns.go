package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	GET_ALL_CAMPAIGNS_QUERY = "SELECT id, name, description, goal, num_raised, num_backers, start_date, end_date, flexible FROM funders.campaign_backers WHERE active = true"
	GET_CAMPAIGN_QUERY      = "SELECT id, name, description, goal, num_raised, num_backers, start_date, end_date, flexible FROM funders.campaign_backers WHERE active = true AND name = $1"
	CAMPAIGN_URL            = "/campaigns"
)

type Campaign struct {
	Id          int64
	Name        string
	Description string
	Goal        float64
	NumRaised   float64
	NumBackers  int64
	StartDate   time.Time
	EndDate     time.Time
	Flexible    bool
}

type Campaigns struct {
	lock   sync.RWMutex
	values map[string]*Campaign
}

func NewCampaigns() *Campaigns {
	campaigns := new(Campaigns)
	campaigns.values = make(map[string]*Campaign)
	return campaigns
}

func (cm Campaigns) AddOrReplaceCampaign(campaign *Campaign) *Campaign {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	cm.values[campaign.Name] = campaign
	return campaign
}

func (cm Campaigns) AddOrReplaceCampaigns(campaigns []*Campaign) {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	for _, campaign := range campaigns {
		cm.values[campaign.Name] = campaign
	}
}

func (cm Campaigns) GetCampaign(name string) (*Campaign, bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	val, exists := cm.values[name]
	return val, exists
}

var campaigns = NewCampaigns()

func getCampaignsFromDb() ([]*Campaign, error) {
	rows, err := db.Query(GET_ALL_CAMPAIGNS_QUERY)
	defer rows.Close()

	var campaigns []*Campaign
	for rows.Next() {
		var campaign Campaign
		err = rows.Scan(&campaign.Id, &campaign.Name, &campaign.Description, &campaign.Goal, &campaign.NumRaised, &campaign.NumBackers, &campaign.StartDate, &campaign.EndDate, &campaign.Flexible)
		if nil == err {
			campaigns = append(campaigns, &campaign)
		} else {
			break
		}
	}

	if nil != err {
		err = rows.Err()
	}

	return campaigns, err
}

func getCampaignFromDb(name string) (Campaign, error) {
	var campaign Campaign
	err := db.QueryRow(GET_CAMPAIGN_QUERY, name).Scan(&campaign.Id, &campaign.Name, &campaign.Description, &campaign.Goal, &campaign.NumRaised, &campaign.NumBackers, &campaign.StartDate, &campaign.EndDate, &campaign.Flexible)
	return campaign, err
}

func getCampaign(name string) (*Campaign, error) {
	var err error
	campaign, exists := campaigns.GetCampaign(name)
	if !exists {
		var campaignDb Campaign
		campaignDb, err = getCampaignFromDb(name)
		campaign = campaigns.AddOrReplaceCampaign(&campaignDb)
		log.Print("Retrieved campaign from database")
	} else {
		log.Print("Retrieved campaign from cache")
	}

	return campaign, err
}

func getCampaignHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	req.Close = true

	var response Response
	campaignName := req.URL.Query().Get("name")

	campaign, err := getCampaign(campaignName)

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
