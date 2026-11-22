package config

import (
	"strings"
	"testing"
)

func TestLoadSiteSource(t *testing.T) {
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
