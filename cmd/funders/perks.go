package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	GET_ALL_PERKS_QUERY = "SELECT id, campaign_id, campaign_name, name, description, price, available, ship_date, num_claimed FROM funders.perk_claims WHERE active = true"
	GET_PERKS_QUERY     = "SELECT id, campaign_id, campaign_name, name, description, price, available, ship_date, num_claimed FROM funders.perk_claims WHERE active = true AND campaign_name = $1"
	PERKS_URL           = "/perks"
)

type Perk struct {
	Id           int64
	CampaignId   int64
	CampaignName string
	Name         string
	Description  string
	Price        float64
	Available    int64
	ShipDate     time.Time
	NumClaimed   int64
}

type Perks struct {
	lock   sync.RWMutex
	values map[string][]*Perk
}

func NewPerks() *Perks {
	perks := new(Perks)
	perks.values = make(map[string][]*Perk)
	return perks
}

func (pks Perks) AddOrReplacePerks(perks []*Perk) {
	pks.lock.Lock()
	defer pks.lock.Unlock()
	for _, perk := range perks {
		pks.values[perk.CampaignName] = perks
	}
}

func (pks Perks) GetPerks(name string) ([]*Perk, bool) {
	pks.lock.RLock()
	defer pks.lock.RUnlock()
	val, exists := pks.values[name]
	return val, exists
}

var perks = NewPerks()

func getPerksFromDb(args ...string) ([]*Perk, error) {
	var rows *sql.Rows
	var err error

	switch len(args) {
	case 0:
		rows, err = db.Query(GET_ALL_PERKS_QUERY)
		break
	case 1:
		rows, err = db.Query(GET_PERKS_QUERY, args[0])
		break
	default:
		return nil, errors.New("Bad parameters used")
	}

	defer rows.Close()

	var perks []*Perk
	for rows.Next() {
		var perk Perk
		err = rows.Scan(&perk.Id, &perk.CampaignId, &perk.CampaignName, &perk.Name, &perk.Description, &perk.Price, &perk.Available, &perk.ShipDate, &perk.NumClaimed)
		if nil == err {
			perks = append(perks, &perk)
		} else {
			break
		}
	}

	if nil != err {
		err = rows.Err()
	}

	return perks, err
}

func getPerks(name string) ([]*Perk, error) {
	var err error
	pks, exists := perks.GetPerks(name)
	if !exists {
		pks, err = getPerksFromDb(name)
		perks.AddOrReplacePerks(pks)
		log.Print("Retrieved perks from database")
	} else {
		log.Print("Retrieved perks from cache")
	}

	return pks, err
}

func getPerkHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	req.Close = true

	var response Response
	campaignName := req.URL.Query().Get("campaign_name")

	perks, err := getPerks(campaignName)

	if nil != err {
		responseStr := "Could not get perks due to server error"
		response = Response{Code: http.StatusInternalServerError, Message: responseStr}
		log.Print(err)
	} else if len(perks) <= 0 {
		responseStr := fmt.Sprintf("%s not found", campaignName)
		response = Response{Code: http.StatusNotFound, Message: responseStr}
		log.Print(responseStr)
	} else {
		jsonStr, _ := json.Marshal(perks)
		return http.StatusOK, string(jsonStr)
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}
