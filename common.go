package common

import (
	"database/sql"
	"os"
)

const (
	DB_DRIVER = "postgres"
)

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
