package main

import (
	"bufio"
	"database/sql"
	_ "flag"
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
	ADD_PROJECT_QUERY = "INSERT INTO funders.projects (name, description, goal, start_date, end_date, ship_date, created_at, updated_at) VALUES($1, $2, $3, $4, $5, $6, $7, $8)"
	ADD_PERK_QUERY    = "INSERT INTO funders.perks (project_id, name, description, price, available, created_at, updated_at) VALUES((SELECT id FROM funders.projects WHERE name = $1), $2, $3, $4, $5, $6, $7)"
	RM_PROJECT_QUERY  = "DELETE FROM funders.projects WHERE name = $1"
	RM_PERK_QUERY     = "DELETE FROM funders.perks WHERE id = $1 AND project_id IN (SELECT id FROM funders.projects WHERE name = $2)"
	TIME_LAYOUT       = "2006-01-02"
)

func getProjectFromCommandLine() (common.Project, error) {
	var (
		project      common.Project
		err          error
		goalStr      string
		startDateStr string
		endDateStr   string
		shipDateStr  string
	)

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter project name (single word): ")
	project.Name, err = reader.ReadString('\n')
	project.Name = strings.TrimSpace(project.Name)

	fmt.Print("Enter project description: ")
	project.Description, err = reader.ReadString('\n')
	project.Description = strings.TrimSpace(project.Description)

	fmt.Print("Enter project goal (numeric value): ")
	goalStr, err = reader.ReadString('\n')
	project.Goal, err = strconv.ParseFloat(strings.TrimSpace(goalStr), 64)

	fmt.Print("Enter project start date (e.g. 2016-09-04): ")
	startDateStr, err = reader.ReadString('\n')
	project.StartDate, err = time.Parse(TIME_LAYOUT, strings.TrimSpace(startDateStr))

	fmt.Print("Enter project end date (e.g. 2016-09-04): ")
	endDateStr, err = reader.ReadString('\n')
	project.EndDate, err = time.Parse(TIME_LAYOUT, strings.TrimSpace(endDateStr))

	fmt.Print("Enter project ship date (e.g. 2016-09-04): ")
	shipDateStr, err = reader.ReadString('\n')
	project.ShipDate, err = time.Parse(TIME_LAYOUT, strings.TrimSpace(shipDateStr))

	return project, err
}

func getPerkFromCommandLine() (common.Perk, error) {
	var (
		perk         common.Perk
		err          error
		priceStr     string
		availableStr string
	)

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter project name (single word): ")
	perk.ProjectName, err = reader.ReadString('\n')
	perk.ProjectName = strings.TrimSpace(perk.ProjectName)

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

	return perk, err
}

func addProjectToDatabase(db *sql.DB, project *common.Project) error {
	return db.QueryRow(ADD_PROJECT_QUERY, project.Name, project.Description, project.Goal, project.StartDate, project.EndDate, project.ShipDate, time.Now(), time.Now()).Scan(&project.Id)
}

func addPerkToDatabase(db *sql.DB, perk *common.Perk) error {
	return db.QueryRow(ADD_PERK_QUERY, perk.ProjectName, perk.Name, perk.Description, perk.Price, perk.Available, time.Now(), time.Now()).Scan(&perk.Id)
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
	/*
	   	var projectFlag = flag.Bool("-add_project", false, "Description of flag")
	   	var perkFlag = flag.Bool("-add_perk", false, "Description of flag")
	   	flag.Parse()

	   	if *projectFlag {
	   		log.Print("Add project")
	   		project, err := getProjectFromCommandLine()
	           if nil != err {
	               log.Fatal(err)
	           } else {
	               addProjectToDatabase(db, &project)
	           }
	   	} else if *perkFlag {
	   		log.Print("Add perk")
	   		perk, err := getPerkFromCommandLine()
	           if nil != err {
	               log.Fatal(err)
	           } else {
	               addPerkToDatabase(db, &perk)
	           }
	   	}
	*/
	/*
	   		perk, err := getPerkFromCommandLine()
	           if nil != err {
	               log.Fatal(err)
	           } else {
	               err = addPerkToDatabase(db, &perk)
	               if nil != err {
	                   log.Fatal(err)
	               }
	           }
	*/
	project, err := getProjectFromCommandLine()
	if nil != err {
		log.Fatal(err)
	} else {
		err = addProjectToDatabase(db, &project)
		if nil != err {
			log.Fatal(err)
		}
	}
}
