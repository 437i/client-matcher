package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/437i/client-matcher/backend/internal/domain/client"
	"github.com/437i/client-matcher/backend/internal/platform/config"
)

func TestOpenCRM(t *testing.T) {
	const dataset = "../../internal/adapters/outbound/crm/csv/testdata/stage0/crm.csv"
	csvRepo, err := openCRM(config.Config{CRMSource: config.CRMCSV, CSV: config.CSVConfig{CRMPath: dataset}})
	if err != nil {
		t.Fatal(err)
	}
	records, err := csvRepo.Search(t.Context(), "Иван", 50)
	if err != nil || len(records) == 0 {
		t.Fatalf("CSV unavailable: %v", err)
	}
	bitrix, err := openCRM(config.Config{CRMSource: config.CRMBitrix24, Bitrix24WebhookURL: "https://example.invalid/rest/1/synthetic/"})
	if err != nil || bitrix == nil {
		t.Fatalf("Bitrix wiring without CSV: %v", err)
	}
	if _, err := bitrix.GetByID(t.Context(), "001"); !errors.Is(err, client.ErrInvalidCRMID) {
		t.Fatalf("unexpected CRM reader: %v", err)
	}
	portal := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "synthetic outage")
	}))
	url := portal.URL + "/rest/1/synthetic/"
	portal.Close()
	bitrix, err = openCRM(config.Config{CRMSource: config.CRMBitrix24, Bitrix24WebhookURL: url, CSV: config.CSVConfig{CRMPath: dataset}})
	if err != nil {
		t.Fatal(err)
	}
	if records, err := bitrix.Search(t.Context(), "Иван", 50); err == nil || records != nil {
		t.Fatalf("expected source error and no CSV fallback, got %d records", len(records))
	}
}
