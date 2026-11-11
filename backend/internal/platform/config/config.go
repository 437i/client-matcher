package config

import (
	"errors"
	"fmt"
	"os"
)

type Config struct {
	DB     DBConfig
	Server ServerConfig
	CSV    CSVConfig
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

	cfg := Config{
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
			CRMPath:  r.required("CRM_PATH"),
			SitePath: r.required("SITE_PATH"),
		},
	}
	if err := errors.Join(r.errs...); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
