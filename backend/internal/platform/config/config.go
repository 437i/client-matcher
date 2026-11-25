package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	CRMCSV          = "csv"
	CRMBitrix24     = "bitrix24"
	SiteCSV         = "csv"
	SitePostgres    = "postgres"
	TransportDirect = "direct"
	TransportSSH    = "ssh"
)

type Config struct {
	CRMSource          string
	Bitrix24WebhookURL string
	SiteSource         string
	DBTransport        string
	SiteSSH            SSHConfig
	ODSSH              SSHConfig
	KnownHostsPath     string
	SiteDB             DBConfig
	DB                 DBConfig
	Server             ServerConfig
	CSV                CSVConfig
}

type SSHConfig struct {
	Address  string
	User     string
	Password string
	KeyPath  string
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
		CRMSource:   r.optional("CRM_SOURCE", CRMCSV),
		SiteSource:  source,
		DBTransport: r.optional("DB_TRANSPORT", TransportDirect),
		Server:      ServerConfig{Addr: ":" + r.optional("PORT", "8080")},
	}
	switch cfg.CRMSource {
	case CRMCSV:
		cfg.CSV.CRMPath = r.required("CRM_PATH")
	case CRMBitrix24:
		cfg.Bitrix24WebhookURL = r.required("BITRIX24_WEBHOOK_URL")
		if cfg.Bitrix24WebhookURL != "" {
			if err := ValidateBitrix24WebhookURL(cfg.Bitrix24WebhookURL); err != nil {
				r.errs = append(r.errs, err)
			}
		}
	default:
		r.errs = append(r.errs, errors.New("CRM_SOURCE must be csv or bitrix24"))
	}
	if source != SiteCSV && source != SitePostgres {
		r.errs = append(r.errs, errors.New("SITE_SOURCE must be csv or postgres"))
	}
	switch cfg.DBTransport {
	case TransportDirect:
		cfg.DB = r.database("DB_", false)
		cfg.DB.SSLMode = r.optional("DB_SSLMODE", "prefer")
		switch source {
		case SiteCSV:
			cfg.CSV.SitePath = r.required("SITE_PATH")
		case SitePostgres:
			cfg.SiteDB = r.database("SITE_DB_", false)
			cfg.SiteDB.SSLMode = r.required("SITE_DB_SSLMODE")
		}
	case TransportSSH:
		if source == SiteCSV {
			r.errs = append(r.errs, errors.New("DB_TRANSPORT=ssh requires SITE_SOURCE=postgres"))
		}
		cfg.SiteDB = r.database("SITE_", true)
		cfg.DB = r.database("OD_", true)
		cfg.KnownHostsPath = r.required("SSH_KNOWN_HOSTS_PATH")
		cfg.SiteSSH = r.ssh("SITE_SSH_")
		cfg.SiteSSH.Password = r.required("SITE_SSH_PASSWORD")
		cfg.ODSSH = r.ssh("OD_SSH_")
		cfg.ODSSH.KeyPath = r.required("OD_SSH_KEY")
	default:
		r.errs = append(r.errs, errors.New("DB_TRANSPORT must be direct or ssh"))
	}
	if err := errors.Join(r.errs...); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func ValidateBitrix24WebhookURL(raw string) error {
	invalid := errors.New("BITRIX24_WEBHOOK_URL must be an HTTPS classic webhook URL")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
		strings.ContainsAny(raw, "%?#\\") {
		return invalid
	}
	parts := strings.Split(u.Path, "/")
	if len(parts) != 5 || parts[0] != "" || parts[1] != "rest" || parts[4] != "" {
		return invalid
	}
	id, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil || id == 0 || parts[3] == "" || parts[3] == "." || parts[3] == ".." || u.EscapedPath() != u.Path {
		return invalid
	}
	return nil
}

func (r *envReader) database(prefix string, tunnel bool) DBConfig {
	cfg := DBConfig{
		Host: r.required(prefix + "HOST"), Name: r.required(prefix + "NAME"),
		User: r.required(prefix + "USER"), Password: r.required(prefix + "PASSWORD"),
	}
	if tunnel {
		cfg.Port, cfg.SSLMode = "5432", "disable"
		if cfg.Host != "" && cfg.Host != "localhost" {
			r.errs = append(r.errs, fmt.Errorf("%sHOST must be localhost", prefix))
		}
	} else {
		cfg.Port = r.required(prefix + "PORT")
	}
	return cfg
}

func (r *envReader) ssh(prefix string) SSHConfig {
	host := r.required(prefix + "HOST")
	if host != "" && (strings.ContainsAny(host, "[]/ \t\r\n") || strings.Contains(host, ":") && net.ParseIP(host) == nil) {
		r.errs = append(r.errs, fmt.Errorf("%sHOST must be a host without a port", prefix))
	}
	return SSHConfig{Address: net.JoinHostPort(host, "22"), User: r.required(prefix + "USER")}
}
