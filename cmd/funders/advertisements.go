package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

const (
	GET_ALL_ADVERTISEMENTS_QUERY = "SELECT campaign_id, campaign_name, perk_id, payment_id, full_name, advertise_other FROM funders.advertisements WHERE advertise = TRUE"
	GET_ADVERTISEMENTS_QUERY     = "SELECT campaign_id, campaign_name, perk_id, payment_id, full_name, advertise_other FROM funders.advertisements WHERE advertise = TRUE AND campaign_name = $1"
	ADVERTISEMENTS_URL           = "/advertisements"
)

type Advertisement struct {
	CampaignId    int64  `json:"campaignId"`
	CampaignName  string `json:"campaignName"`
	PerkId        int64  `json:"perkId"`
	PaymentId     string `json:"paymentId"`
	AdvertiseName string `json:"advertiseName"`
}

type Advertisements struct {
	lock       sync.RWMutex
	nameValues map[string][]*Advertisement
}

func NewAdvertisements() *Advertisements {
	advertisements := new(Advertisements)
	advertisements.nameValues = make(map[string][]*Advertisement)
	return advertisements
}

func (ads *Advertisements) AddOrReplaceAdvertisements(advertisements []*Advertisement) {
	ads.lock.Lock()
	defer ads.lock.Unlock()
	for _, advertisement := range advertisements {
		if _, exists := ads.nameValues[advertisement.CampaignName]; !exists {
			ads.nameValues[advertisement.CampaignName] = make([]*Advertisement, 0)
		}
		ads.nameValues[advertisement.CampaignName] = append(ads.nameValues[advertisement.CampaignName], advertisement)
	}
}

func (ads *Advertisements) AddAdvertisement(campaignName string, payment *Payment) {
	if payment.Advertise && payment.State == "success" {
		var advertisement Advertisement

		advertisement.CampaignId = payment.CampaignId
		advertisement.CampaignName = campaignName
		advertisement.PerkId = payment.PerkId
		advertisement.PaymentId = payment.Id

		advertiseOther := strings.TrimSpace(payment.AdvertiseOther)
		if len(advertiseOther) > 0 {
			advertisement.AdvertiseName = payment.AdvertiseOther
		} else {
			advertisement.AdvertiseName = payment.FullName
		}

		ads.lock.Lock()
		defer ads.lock.Unlock()
		ads.nameValues[advertisement.CampaignName] = append(ads.nameValues[advertisement.CampaignName], &advertisement)
	}
}

func (ads *Advertisements) GetAdvertisements(name string) ([]*Advertisement, bool) {
	ads.lock.RLock()
	defer ads.lock.RUnlock()
	val, exists := ads.nameValues[name]
	return val, exists
}

var advertisements = NewAdvertisements()

func getAdvertisementsFromDb(args ...string) ([]*Advertisement, error) {
	var rows *sql.Rows
	var err error

	switch len(args) {
	case 0:
		rows, err = db.Query(GET_ALL_ADVERTISEMENTS_QUERY)
		break
	case 1:
		rows, err = db.Query(GET_ADVERTISEMENTS_QUERY, args[0])
		break
	default:
		return nil, errors.New("Bad parameters used")
	}

	if nil != err {
		return nil, err
	}

	defer rows.Close()

	var advertisements []*Advertisement
	for rows.Next() {
		var advertisement Advertisement
		var advertiseOther sql.NullString
		err = rows.Scan(&advertisement.CampaignId, &advertisement.CampaignName, &advertisement.PerkId, &advertisement.PaymentId, &advertisement.AdvertiseName, &advertiseOther)
		if nil == err {
			if advertiseOther.Valid {
				advertisement.AdvertiseName = advertiseOther.String
			}
			advertisements = append(advertisements, &advertisement)
		} else {
			break
		}
	}

	if nil == err {
		err = rows.Err()
	}

	return advertisements, err
}

func getAdvertisements(name string) ([]*Advertisement, error) {
	var err error
	ads, exists := advertisements.GetAdvertisements(name)
	if !exists {
		ads, err = getAdvertisementsFromDb(name)
		if nil == err {
			advertisements.AddOrReplaceAdvertisements(ads)
			log.Print("Retrieved advertisements from database")
		} else {
			log.Print("Advertisements not found in database")
		}
	} else {
		log.Print("Retrieved advertisements from cache")
	}

	return ads, err
}

func getAdvertisementHandler(res http.ResponseWriter, req *http.Request) (int, string) {
	res.Header().Set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE)
	req.Close = true

	var response Response
	campaignName := strings.TrimSpace(req.URL.Query().Get("campaign_name"))

	if len(campaignName) == 0 {
		responseStr := "Campaign name parameter required"
		response = Response{Code: http.StatusBadRequest, Message: responseStr}
	} else {
		advertisements, err := getAdvertisements(campaignName)

		if nil != err {
			responseStr := "Could not get advertisements due to server error"
			response = Response{Code: http.StatusInternalServerError, Message: responseStr}
			log.Print(err)
		} else if len(advertisements) <= 0 {
			responseStr := fmt.Sprintf("%s not found", campaignName)
			response = Response{Code: http.StatusNotFound, Message: responseStr}
			log.Print(responseStr)
		} else {
			jsonStr, _ := json.Marshal(advertisements)
			return http.StatusOK, string(jsonStr)
		}
	}

	jsonStr, _ := json.Marshal(response)
	return response.Code, string(jsonStr)
}
