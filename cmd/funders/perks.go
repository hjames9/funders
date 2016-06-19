package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hjames9/funders"
	"log"
	"net/http"
	"strings"
	"sync"
)

const (
	GET_ALL_PERKS_QUERY = "SELECT id, campaign_id, campaign_name, name, description, price, currency, available_for_payment, available_for_pledge, ship_date, num_claimed, num_pledged FROM funders.perk_claims WHERE active = TRUE"
	GET_PERKS_QUERY     = "SELECT id, campaign_id, campaign_name, name, description, price, currency, available_for_payment, available_for_pledge, ship_date, num_claimed, num_pledged FROM funders.perk_claims WHERE active = TRUE AND campaign_name = $1"
	PERKS_URL           = "/perks"
)

type Perk common.Perk

func (perk *Perk) IsAvailableForPayment() bool {
	return perk.AvailableForPayment > perk.NumClaimed
}

func (perk *Perk) IsAvailableForPledge() bool {
	return perk.AvailableForPledge > perk.NumPledged
}

func (perk *Perk) IncrementNumClaimed(amount int64) int64 {
	perk.Lock.Lock()
	defer perk.Lock.Unlock()
	perk.NumClaimed += amount
	return perk.NumClaimed
}

func (perk *Perk) IncrementNumPledged(amount int64) int64 {
	perk.Lock.Lock()
	defer perk.Lock.Unlock()
	perk.NumPledged += amount
	return perk.NumPledged
}

func (perk *Perk) MarshalJSON() ([]byte, error) {
	perk.Lock.RLock()
	numClaimed := perk.NumClaimed
	numPledged := perk.NumPledged
	perk.Lock.RUnlock()

	type MyPerk Perk
	return json.Marshal(&struct {
		NumClaimed int64 `json:"numClaimed"`
		NumPledged int64 `json:"numPledged"`
		*MyPerk
	}{
		NumClaimed: numClaimed,
		NumPledged: numPledged,
		MyPerk:     (*MyPerk)(perk),
	})
}

type Perks struct {
	lock       sync.RWMutex
	nameValues map[string][]*Perk
	idValues   map[int64]*Perk
}

func NewPerks() *Perks {
	perks := new(Perks)
	perks.nameValues = make(map[string][]*Perk)
	perks.idValues = make(map[int64]*Perk)
	return perks
}

func (pks *Perks) AddOrReplacePerks(perks []*Perk) {
	pks.lock.Lock()
	defer pks.lock.Unlock()
	for _, perk := range perks {
		if _, exists := pks.nameValues[perk.CampaignName]; !exists {
			pks.nameValues[perk.CampaignName] = make([]*Perk, 0)
		}
		pks.nameValues[perk.CampaignName] = append(pks.nameValues[perk.CampaignName], perk)
		pks.idValues[perk.Id] = perk
	}
}

func (pks *Perks) GetPerks(name string) ([]*Perk, bool) {
	pks.lock.RLock()
	defer pks.lock.RUnlock()
	val, exists := pks.nameValues[name]
	return val, exists
}

func (pks *Perks) GetPerk(id int64) (*Perk, bool) {
	pks.lock.RLock()
	defer pks.lock.RUnlock()
	val, exists := pks.idValues[id]
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

	if nil != err {
		return nil, err
	}

	defer rows.Close()

	var perks []*Perk
	for rows.Next() {
		var perk Perk
		err = rows.Scan(&perk.Id, &perk.CampaignId, &perk.CampaignName, &perk.Name, &perk.Description, &perk.Price, &perk.Currency, &perk.AvailableForPayment, &perk.AvailableForPledge, &perk.ShipDate, &perk.NumClaimed, &perk.NumPledged)
		if nil == err {
			perks = append(perks, &perk)
		} else {
			break
		}
	}

	if nil == err {
		err = rows.Err()
	}

	return perks, err
}

func getPerks(name string) ([]*Perk, error) {
	var err error
	pks, exists := perks.GetPerks(name)
	if !exists {
		pks, err = getPerksFromDb(name)
		if nil == err {
			perks.AddOrReplacePerks(pks)
			log.Print("Retrieved perks from database")
		} else {
			log.Print("Perks not found in database")
		}
	} else {
		log.Print("Retrieved perks from cache")
	}

	return pks, err
}

func getPerkHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	req.Close = true

	var response common.Response
	campaignName := strings.TrimSpace(req.URL.Query().Get("campaign_name"))

	if len(campaignName) == 0 {
		responseStr := "Campaign name parameter required"
		response = common.Response{Code: http.StatusBadRequest, Message: responseStr}
	} else {
		perks, err := getPerks(campaignName)

		if nil != err {
			responseStr := "Could not get perks due to server error"
			response = common.Response{Code: http.StatusInternalServerError, Message: responseStr}
			log.Print(err)
		} else if len(perks) <= 0 {
			responseStr := fmt.Sprintf("%s not found", campaignName)
			response = common.Response{Code: http.StatusNotFound, Message: responseStr}
			log.Print(responseStr)
		} else {
			jsonStr, _ := json.Marshal(perks)
			return http.StatusOK, string(jsonStr)
		}
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}
