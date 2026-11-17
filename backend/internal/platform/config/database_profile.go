package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const localDockerDatabasePassword = "pm-local"

func developmentDatabaseDSN(environment, driver, configuredDSN string) string {
	if strings.EqualFold(environment, "production") || driver != "postgres" {
		return configuredDSN
	}
	return assembleLocalDockerDSN()
}

func assembleLocalDockerDSN() string {
	host := "127.0.0.1"
	port := env("PM_DEV_DB_PORT", "5433")
	user := env("PM_DEV_DB_USER", "pm")
	password := env("PM_DEV_DB_PASSWORD", localDockerDatabasePassword)
	name := env("PM_DEV_DB_NAME", "pm_dev")
	timezone := env("PM_TIMEZONE", "Asia/Shanghai")
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=%s",
		host, port, user, password, name, timezone,
	)
}

func (c Config) validateDevelopmentDatabaseTarget() error {
	if strings.EqualFold(c.Environment, "production") {
		return nil
	}
	if c.DatabaseDriver != "postgres" {
		return nil
	}
	switch strings.ToLower(postgresDatabaseName(c.DatabaseDSN)) {
	case "pm", "pm_prod", "pm-prod", "pm_main", "pm-main":
		return errors.New("development database cannot target production; use PM_DEV_DB_NAME=pm_dev and pnpm db:refresh")
	}
	return nil
}

func postgresDatabaseName(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			return ""
		}
		return strings.TrimPrefix(parsed.Path, "/")
	}
	for _, field := range strings.Fields(dsn) {
		key, value, ok := strings.Cut(field, "=")
		if ok && key == "dbname" {
			return value
		}
	}
	return ""
}
