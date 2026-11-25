package config

import (
	"strings"
	"testing"

	"github.com/bitrix24/b24gosdk/b24test"
)

func TestLoadSiteSource(t *testing.T) {
	t.Setenv("CRM_SOURCE", "csv")
	t.Setenv("DB_TRANSPORT", "")
	for _, key := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "CRM_PATH", "SITE_PATH"} {
		t.Setenv(key, "synthetic")
	}
	siteKeys := []string{"SITE_DB_HOST", "SITE_DB_PORT", "SITE_DB_USER", "SITE_DB_PASSWORD", "SITE_DB_NAME", "SITE_DB_SSLMODE"}
	for _, key := range append(siteKeys, "SITE_SOURCE") {
		t.Setenv(key, "")
	}
	cfg, err := Load()
	if err != nil || cfg.SiteSource != SiteCSV || cfg.CSV.SitePath != "synthetic" || cfg.SiteDB != (DBConfig{}) {
		t.Fatalf("default CSV settings: %v", err)
	}
	t.Setenv("SITE_PATH", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SITE_PATH") || strings.Contains(err.Error(), "SITE_DB_") {
		t.Fatalf("CSV validation: %v", err)
	}
	t.Setenv("SITE_SOURCE", SitePostgres)
	if _, err := Load(); err == nil {
		t.Fatal("missing site DB configuration accepted")
	} else {
		for _, key := range siteKeys {
			if !strings.Contains(err.Error(), key) {
				t.Errorf("missing error for %s", key)
			}
		}
		if strings.Contains(err.Error(), "SITE_PATH") {
			t.Error("postgres should not require SITE_PATH")
		}
	}
	for _, key := range siteKeys {
		t.Setenv(key, "site-synthetic")
	}
	cfg, err = Load()
	if err != nil || cfg.SiteSource != SitePostgres || cfg.SiteDB.Host != "site-synthetic" || cfg.DB.Host != "synthetic" {
		t.Fatalf("postgres settings: %v", err)
	}
	t.Setenv("SITE_SOURCE", "unknown")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SITE_SOURCE") {
		t.Fatalf("source validation: %v", err)
	}
}

func TestLoadSSH(t *testing.T) {
	t.Setenv("CRM_SOURCE", "csv")
	t.Setenv("DB_TRANSPORT", "ssh")
	t.Setenv("SITE_SOURCE", "postgres")
	for _, key := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "SITE_PATH"} {
		t.Setenv(key, "")
	}
	required := []string{"CRM_PATH", "SSH_KNOWN_HOSTS_PATH", "SITE_SSH_HOST", "SITE_SSH_USER", "SITE_SSH_PASSWORD", "OD_SSH_HOST", "OD_SSH_USER", "OD_SSH_KEY", "SITE_HOST", "SITE_NAME", "SITE_USER", "SITE_PASSWORD", "OD_HOST", "OD_NAME", "OD_USER", "OD_PASSWORD"}
	for _, key := range required {
		t.Setenv(key, "synthetic-value")
	}
	t.Setenv("SITE_HOST", "localhost")
	t.Setenv("OD_HOST", "localhost")
	cfg, err := Load()
	if err != nil || cfg.ODSSH.Address != "synthetic-value:22" || cfg.SiteSSH.Address != "synthetic-value:22" || cfg.DB.Port != "5432" || cfg.SiteDB.Port != "5432" || cfg.DB.SSLMode != "disable" || cfg.SiteDB.SSLMode != "disable" {
		t.Fatalf("invalid SSH configuration: %v", err)
	}
	for _, key := range required {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "")
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), key) || strings.Contains(err.Error(), "synthetic-value") {
				t.Fatalf("validation for %s: %v", key, err)
			}
		})
	}
	for _, tc := range []struct{ key, value string }{
		{"SITE_SOURCE", ""}, {"SITE_SOURCE", "csv"}, {"SITE_SOURCE", "invalid"},
		{"DB_TRANSPORT", "invalid"}, {"SITE_HOST", "127.0.0.1"}, {"OD_HOST", "other"},
		{"SITE_SSH_HOST", "host:2022"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("validation for %s: %v", tc.key, err)
			}
		})
	}
}

func TestLoadCRMSource(t *testing.T) {
	for _, key := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "CRM_PATH", "SITE_PATH"} {
		t.Setenv(key, "synthetic")
	}
	t.Setenv("SITE_SOURCE", "csv")
	t.Setenv("DB_TRANSPORT", "direct")
	t.Setenv("BITRIX24_WEBHOOK_URL", "invalid")
	for _, source := range []string{"", "csv"} {
		t.Setenv("CRM_SOURCE", source)
		cfg, err := Load()
		if err != nil || cfg.CRMSource != CRMCSV || cfg.CSV.CRMPath != "synthetic" || cfg.Bitrix24WebhookURL != "" {
			t.Fatalf("CSV selection: %v", err)
		}
	}
	t.Setenv("CRM_PATH", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "CRM_PATH") {
		t.Fatal("missing CRM_PATH accepted")
	}
	t.Setenv("CRM_SOURCE", "invalid")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "CRM_SOURCE") {
		t.Fatal("invalid CRM_SOURCE accepted")
	}
	t.Setenv("CRM_SOURCE", "bitrix24")
	t.Setenv("BITRIX24_WEBHOOK_URL", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "BITRIX24_WEBHOOK_URL") {
		t.Fatal("missing webhook accepted")
	}
	t.Setenv("BITRIX24_WEBHOOK_URL", "https://example.invalid/rest/1/synthetic/")
	cfg, err := Load()
	if err != nil || cfg.CRMSource != CRMBitrix24 || cfg.CSV.CRMPath != "" || cfg.Bitrix24WebhookURL == "" {
		t.Fatalf("Bitrix selection: %v", err)
	}
}

func TestWebhookURLValidation(t *testing.T) {
	portal := b24test.NewPortal(t)
	for _, raw := range []string{
		portal.URL(), "https:///rest/1/synthetic/", "https://synthetic@example.invalid/rest/1/synthetic/",
		"https://example.invalid/rest/1/synthetic/?", "https://example.invalid/rest/1/synthetic/#",
		"https://example.invalid/rest/api/synthetic/", "https://example.invalid/rest/0/synthetic/",
		"https://example.invalid/rest/-1/synthetic/", "https://example.invalid/rest/1/%73ynthetic/",
		"https://example.invalid/rest/1/synthetic", "https://example.invalid/rest/1//",
		"https://example.invalid/rest/1/../", "https://example.invalid/rest/1/./",
		"https://example.invalid/rest/1/synthetic/extra/", "https://example.invalid/rest/1/syn thetic/",
	} {
		if err := ValidateBitrix24WebhookURL(raw); err == nil || strings.Contains(err.Error(), raw) || strings.Contains(err.Error(), "synthetic") {
			t.Fatal("expected safe validation error")
		}
	}
}
