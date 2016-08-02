package common

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	DB_DRIVER         = "postgres"
	TIME_LAYOUT       = "2006-01-02"
	USER_AGENT_HEADER = "User-Agent"
	XFP_HEADER        = "X-Forwarded-Proto"
)

type Campaign struct {
	Id          int64        `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Goal        float64      `json:"goal"`
	Currency    string       `json:"currency"`
	AmtRaised   float64      `json:"-"`
	NumBackers  int64        `json:"-"`
	AmtPledged  float64      `json:"-"`
	NumPledgers int64        `json:"-"`
	StartDate   time.Time    `json:"startDate"`
	EndDate     time.Time    `json:"endDate"`
	Flexible    bool         `json:"flexible"`
	Lock        sync.RWMutex `json:"-"`
}

type Perk struct {
	Id                  int64        `json:"id"`
	CampaignId          int64        `json:"campaignId"`
	CampaignName        string       `json:"campaignName"`
	Name                string       `json:"name"`
	Description         string       `json:"description"`
	Price               float64      `json:"price"`
	Currency            string       `json:"currency"`
	AvailableForPayment int64        `json:"availableForPayment"`
	AvailableForPledge  int64        `json:"availableForPledge"`
	ShipDate            time.Time    `json:"shipDate"`
	NumClaimed          int64        `json:"-"`
	NumPledged          int64        `json:"-"`
	Lock                sync.RWMutex `json:"-"`
}

type Response struct {
	Code    int
	Message string
	Id      string `json:",omitempty"`
}

type ErrorType int

const (
	BadRequestError ErrorType = 1 << iota
	NotFoundError
	ServerError
	ServiceUnavailableError
	ServiceNotImplementedError
)

type RequestError struct {
	Message string
	Type    ErrorType
}

func (requestError RequestError) Error() string {
	return fmt.Sprintf("%d: %s", requestError.Type, requestError.Message)
}

func GetenvWithDefault(envKey string, defaultVal string) string {
	envVal := os.Getenv(envKey)

	if len(envVal) == 0 {
		envVal = defaultVal
	}

	return envVal
}

func CreateSqlString(value string) sql.NullString {
	var nullValue sql.NullString
	if len(value) > 0 {
		nullValue = sql.NullString{value, true}
	}
	return nullValue
}

func GetScheme(request *http.Request) string {
	prot := request.Header.Get(XFP_HEADER)
	if len(prot) > 0 {
		return prot
	}

	if nil == request.TLS {
		return "http"
	} else {
		return "https"
	}
}
