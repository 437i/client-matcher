package config

import (
	"errors"
	"fmt"
	"os"
)

const (
	SiteCSV      = "csv"
	SitePostgres = "postgres"
)

type Config struct {
	SiteSource string
	SiteDB     DBConfig
	DB         DBConfig
	Server     ServerConfig
	CSV        CSVConfig
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type ServerConfig struct {
	Addr string
}

type CSVConfig struct {
	CRMPath  string
	SitePath string
}

type envReader struct {
	errs []error
}

func (r *envReader) required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		r.errs = append(r.errs, fmt.Errorf("%s is required", key))
	}
	return v
}

func (r *envReader) optional(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() (Config, error) {
	r := &envReader{}
	source := r.optional("SITE_SOURCE", SiteCSV)

	cfg := Config{
		SiteSource: source,
		DB: DBConfig{
			Host:     r.required("DB_HOST"),
			Port:     r.required("DB_PORT"),
			User:     r.required("DB_USER"),
			Password: r.required("DB_PASSWORD"),
			Name:     r.required("DB_NAME"),
			SSLMode:  r.optional("DB_SSLMODE", "prefer"),
		},
		Server: ServerConfig{
			Addr: ":" + r.optional("PORT", "8080"),
		},
		CSV: CSVConfig{
			CRMPath: r.required("CRM_PATH"),
		},
	}
	switch source {
	case SiteCSV:
		cfg.CSV.SitePath = r.required("SITE_PATH")
	case SitePostgres:
		cfg.SiteDB = DBConfig{
			Host: r.required("SITE_DB_HOST"), Port: r.required("SITE_DB_PORT"),
			User: r.required("SITE_DB_USER"), Password: r.required("SITE_DB_PASSWORD"),
			Name: r.required("SITE_DB_NAME"), SSLMode: r.required("SITE_DB_SSLMODE"),
		}
	default:
		r.errs = append(r.errs, errors.New("SITE_SOURCE must be csv or postgres"))
	}
	if err := errors.Join(r.errs...); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
