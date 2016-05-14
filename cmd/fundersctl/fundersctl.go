package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"github.com/hjames9/funders"
	_ "github.com/lib/pq"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ADD_CAMPAIGN_QUERY    = "INSERT INTO funders.campaigns (name, description, goal, start_date, end_date, flexible, created_at, updated_at) VALUES($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id"
	ADD_PERK_QUERY        = "INSERT INTO funders.perks (campaign_id, name, description, price, available, ship_date, created_at, updated_at) VALUES((SELECT id FROM funders.campaigns WHERE name = $1), $2, $3, $4, $5, $6, $7, $8) RETURNING id"
	RM_CAMPAIGN_QUERY     = "DELETE FROM funders.campaigns WHERE name = $1"
	RM_PERK_QUERY         = "DELETE FROM funders.perks WHERE name = $1 AND campaign_id IN (SELECT id FROM funders.campaigns WHERE name = $2)"
	UPDATE_CAMPAIGN_QUERY = "UPDATE funders.campaigns SET updated_at = $1, ? WHERE name = ?"
	UPDATE_PERK_QUERY     = "UPDATE funders.perks SET updated_at = $1, ? WHERE name = ? AND campaign_id IN (SELECT id FROM funders.campaigns WHERE name = ?)"
	ACTIVE_CAMPAIGN_QUERY = "UPDATE funders.campaigns SET updated_at = $1, active = $2 WHERE name = $3"
	ACTIVE_PERK_QUERY     = "UPDATE funders.perks SET updated_at = $1, active = $2 WHERE name = $3 AND campaign_id IN (SELECT id FROM funders.campaigns WHERE name = $4)"
	TIME_LAYOUT           = "2006-01-02"
)

func getCampaignFromCommandLine() (common.Campaign, error) {
	var (
		campaign     common.Campaign
		err          error
		goalStr      string
		startDateStr string
		endDateStr   string
		flexibleStr  string
	)

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter campaign name (single word): ")
	campaign.Name, err = reader.ReadString('\n')
	campaign.Name = strings.TrimSpace(campaign.Name)

	fmt.Print("Enter campaign description: ")
	campaign.Description, err = reader.ReadString('\n')
	campaign.Description = strings.TrimSpace(campaign.Description)

	fmt.Print("Enter campaign goal (numeric value): ")
	goalStr, err = reader.ReadString('\n')
	campaign.Goal, err = strconv.ParseFloat(strings.TrimSpace(goalStr), 64)

	fmt.Print("Enter campaign start date (e.g. 2016-09-04): ")
	startDateStr, err = reader.ReadString('\n')
	campaign.StartDate, err = time.Parse(TIME_LAYOUT, strings.TrimSpace(startDateStr))

	fmt.Print("Enter campaign end date (e.g. 2016-09-04): ")
	endDateStr, err = reader.ReadString('\n')
	campaign.EndDate, err = time.Parse(TIME_LAYOUT, strings.TrimSpace(endDateStr))

	fmt.Print("Enter campaign flexibility (e.g. true): ")
	flexibleStr, err = reader.ReadString('\n')
	campaign.Flexible, err = strconv.ParseBool(strings.TrimSpace(flexibleStr))

	return campaign, err
}

func getPerkFromCommandLine() (common.Perk, error) {
	var (
		perk         common.Perk
		err          error
		priceStr     string
		availableStr string
		shipDateStr  string
	)

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter campaign name (single word): ")
	perk.CampaignName, err = reader.ReadString('\n')
	perk.CampaignName = strings.TrimSpace(perk.CampaignName)

	fmt.Print("Enter perk name: ")
	perk.Name, err = reader.ReadString('\n')
	perk.Name = strings.TrimSpace(perk.Name)

	fmt.Print("Enter perk description: ")
	perk.Description, err = reader.ReadString('\n')
	perk.Description = strings.TrimSpace(perk.Description)

	fmt.Print("Enter perk price: ")
	priceStr, err = reader.ReadString('\n')
	perk.Price, err = strconv.ParseFloat(strings.TrimSpace(priceStr), 64)

	fmt.Print("Enter perk available: ")
	availableStr, err = reader.ReadString('\n')
	perk.Available, err = strconv.ParseInt(strings.TrimSpace(availableStr), 10, 64)

	fmt.Print("Enter perk ship date (e.g. 2016-09-04): ")
	shipDateStr, err = reader.ReadString('\n')
	perk.ShipDate, err = time.Parse(TIME_LAYOUT, strings.TrimSpace(shipDateStr))

	return perk, err
}

func addCampaignToDatabase(db *sql.DB, campaign *common.Campaign) (int64, error) {
	err := db.QueryRow(ADD_CAMPAIGN_QUERY, campaign.Name, campaign.Description, campaign.Goal, campaign.StartDate, campaign.EndDate, campaign.Flexible, time.Now(), time.Now()).Scan(&campaign.Id)
	return campaign.Id, err
}

func addPerkToDatabase(db *sql.DB, perk *common.Perk) (int64, error) {
	err := db.QueryRow(ADD_PERK_QUERY, perk.CampaignName, perk.Name, perk.Description, perk.Price, perk.Available, perk.ShipDate, time.Now(), time.Now()).Scan(&perk.Id)
	return perk.Id, err
}

func getCampaignNameFromCommandLine() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter campaign name: ")
	campaignName, err := reader.ReadString('\n')
	campaignName = strings.TrimSpace(campaignName)

	return campaignName, err
}

func getPerkAndCampaignNameFromCommandLine() (string, string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter campaign name: ")
	campaignName, err := reader.ReadString('\n')
	campaignName = strings.TrimSpace(campaignName)

	fmt.Print("Enter perk name: ")
	perkName, err := reader.ReadString('\n')
	perkName = strings.TrimSpace(perkName)

	return campaignName, perkName, err
}

func removeCampaignFromDatabase(db *sql.DB, campaignName string) error {
	_, err := db.Exec(RM_CAMPAIGN_QUERY, campaignName)
	return err
}

func removePerkFromDatabase(db *sql.DB, campaignName string, perkName string) error {
	_, err := db.Exec(RM_PERK_QUERY, perkName, campaignName)
	return err
}

func createUpdateQueryString(templateQuery string, values map[string]interface{}) (string, []interface{}) {
	var buffer bytes.Buffer
	counter := 0

	parameters := make([]interface{}, 0, len(values))

	for key, value := range values {
		counter++
		buffer.WriteString(fmt.Sprintf("%s = $%d, ", key, counter))
		parameters = append(parameters, value)
	}

	buffer.Truncate(buffer.Len() - 1)

	newQuery := strings.Replace(templateQuery, "?", buffer.String(), 1)
	newQuery = strings.Replace(newQuery, "?", fmt.Sprintf("$%d", counter), 1)

	return newQuery, parameters
}

func updateCampaignFromDatabase(db *sql.DB, values map[string]interface{}, campaignName string) error {
	campaignQuery, parameters := createUpdateQueryString(UPDATE_CAMPAIGN_QUERY, values)
	_, err := db.Exec(campaignQuery, time.Now(), parameters, campaignName)
	return err
}

func updatePerkFromDatabase(db *sql.DB, values map[string]interface{}, campaignName string, perkName string) error {
	perkQuery, parameters := createUpdateQueryString(UPDATE_PERK_QUERY, values)
	_, err := db.Exec(perkQuery, time.Now(), parameters, perkName, campaignName)
	return err
}

func flipActivationForCampaign(db *sql.DB, campaignName string, active bool) error {
	_, err := db.Exec(ACTIVE_CAMPAIGN_QUERY, time.Now(), active, campaignName)
	return err
}

func flipActivationForPerk(db *sql.DB, campaignName string, perkName string, active bool) error {
	_, err := db.Exec(ACTIVE_PERK_QUERY, time.Now(), active, perkName, campaignName)
	return err
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

	db := dbCredentials.GetDatabase()
	defer db.Close()

	//Command line flags
	addCampaignFlag := flag.Bool("add_campaign", false, "Add campaign for crowdfunding")
	addPerkFlag := flag.Bool("add_perk", false, "Add perk for existing campaign")

	rmCampaignFlag := flag.Bool("rm_campaign", false, "Remove campaign for crowdfunding")
	rmPerkFlag := flag.Bool("rm_perk", false, "Remove perk for existing campaign")

	updateCampaignFlag := flag.Bool("up_campaign", false, "Update existing campaign attributes")
	updatePerkFlag := flag.Bool("up_perk", false, "Update perk for existing campaign")

	activateCampaignFlag := flag.Bool("activate_campaign", false, "Activate deactive campaign")
	deactivateCampaignFlag := flag.Bool("deactivate_campaign", false, "Deactivate active campaign")

	activatePerkFlag := flag.Bool("activate_perk", false, "Activate deactive perk")
	deactivatePerkFlag := flag.Bool("deactivate_perk", false, "Deactivate active perk")
	flag.Parse()

	if *addCampaignFlag {
		log.Print("Adding campaign")
		campaign, err := getCampaignFromCommandLine()
		if nil != err {
			log.Fatal(err)
		} else {
			id, err := addCampaignToDatabase(db, &campaign)
			if nil != err {
				log.Fatal(err)
			} else {
				log.Printf("Id is %d", id)
			}
		}
	} else if *addPerkFlag {
		log.Print("Adding perk")
		perk, err := getPerkFromCommandLine()
		if nil != err {
			log.Fatal(err)
		} else {
			id, err := addPerkToDatabase(db, &perk)
			if nil != err {
				log.Fatal(err)
			} else {
				log.Printf("Id is %d", id)
			}
		}
	} else if *rmCampaignFlag {
		log.Print("Removing campaign")
		campaignName, err := getCampaignNameFromCommandLine()
		if nil != err {
			log.Fatal(err)
		} else {
			err := removeCampaignFromDatabase(db, campaignName)
			if nil != err {
				log.Fatal(err)
			}
		}
	} else if *rmPerkFlag {
		log.Print("Removing perk")
		campaignName, perkName, err := getPerkAndCampaignNameFromCommandLine()
		if nil != err {
			log.Fatal(err)
		} else {
			err := removePerkFromDatabase(db, campaignName, perkName)
			if nil != err {
				log.Fatal(err)
			}
		}
	} else if *updateCampaignFlag {
		log.Print("Update campaign")
	} else if *updatePerkFlag {
		log.Print("Update perk")
	} else if *activateCampaignFlag {
		//log.Printf("Activate campaign %s", campaignName)
		//err := flipActivationForCampaign(db, campaignName, true)
	} else if *deactivateCampaignFlag {
		//log.Printf("Deactivate campaign %s", campaignName)
		//err := flipActivationForCampaign(db, campaignName, false)
	} else if *activatePerkFlag {
		//log.Printf("Activate campaign %s", campaignName)
		//err := flipActivationForPerk(db, campaignName, true)
	} else if *deactivatePerkFlag {
		//err := flipActivationForPerk(db, campaignName, false)
	} else {
		flag.Usage()
	}
}
