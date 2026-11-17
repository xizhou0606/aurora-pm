package config

import (
	"strings"
	"testing"
)

func TestDevelopmentRejectsProductionDatabaseName(t *testing.T) {
	cfg := Config{
		Environment: "development", DatabaseDriver: "postgres", AuthMode: "local",
		DatabaseDSN: "host=127.0.0.1 port=5432 user=pm password=secret dbname=pm sslmode=disable",
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "cannot target production") {
		t.Fatalf("production database name was accepted in development: %v", err)
	}
}

func TestDevelopmentAllowsLocalDockerDatabase(t *testing.T) {
	cfg := Config{
		Environment: "development", DatabaseDriver: "postgres", AuthMode: "local",
		DatabaseDSN: "host=127.0.0.1 port=5433 user=pm password=secret dbname=pm_dev sslmode=disable",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("local Docker DSN rejected: %v", err)
	}
}

func TestProductionAllowsProductionDatabaseName(t *testing.T) {
	cfg := Config{
		Environment: "production", DatabaseDriver: "postgres", AuthMode: "local",
		DatabaseDSN: "host=db.internal user=pm password=secret dbname=pm sslmode=require",
		AI:          AI{Enabled: false},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("production DSN rejected: %v", err)
	}
}

func TestPostgresDatabaseNameParsesLibpqAndURL(t *testing.T) {
	if got := postgresDatabaseName("host=127.0.0.1 dbname=pm_dev user=pm"); got != "pm_dev" {
		t.Fatalf("libpq dbname=%q", got)
	}
	if got := postgresDatabaseName("postgres://pm@db.example:5432/pm?sslmode=disable"); got != "pm" {
		t.Fatalf("url dbname=%q", got)
	}
}

func TestDevelopmentDatabaseDSNAssemblesFromDevDB(t *testing.T) {
	t.Setenv("PM_DEV_DB_PASSWORD", "secret")
	t.Setenv("PM_DEV_DB_NAME", "pm_dev")
	t.Setenv("PM_DEV_DB_USER", "pm")
	t.Setenv("PM_DEV_DB_PORT", "5433")
	t.Setenv("PM_TIMEZONE", "Asia/Shanghai")
	want := "host=127.0.0.1 port=5433 user=pm password=secret dbname=pm_dev sslmode=disable TimeZone=Asia/Shanghai"
	got := developmentDatabaseDSN("development", "postgres", "host=remote.example dbname=pm_dev")
	if got != want {
		t.Fatalf("assembled DSN=%q want %q", got, want)
	}
}

func TestDevelopmentDatabaseDSNUsesDefaultPasswordWhenMissing(t *testing.T) {
	t.Setenv("PM_DEV_DB_PASSWORD", "")
	t.Setenv("PM_DEV_DB_NAME", "pm_dev")
	t.Setenv("PM_DEV_DB_USER", "pm")
	t.Setenv("PM_DEV_DB_PORT", "5433")
	t.Setenv("PM_TIMEZONE", "Asia/Shanghai")
	want := "host=127.0.0.1 port=5433 user=pm password=pm-local dbname=pm_dev sslmode=disable TimeZone=Asia/Shanghai"
	got := developmentDatabaseDSN("development", "postgres", "host=remote.example dbname=pm_dev")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestProductionDatabaseDSNNeverAssembles(t *testing.T) {
	t.Setenv("PM_DEV_DB_PASSWORD", "secret")
	configured := "host=db.internal dbname=pm"
	if got := developmentDatabaseDSN("production", "postgres", configured); got != configured {
		t.Fatalf("got %q", got)
	}
}
