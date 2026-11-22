package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/437i/client-matcher/backend/internal/domain/link"
)

var linkCSVHeader = []string{"crm_id", "site_id"}

func writeLinksCSV(w io.Writer, links []link.ClientLink) error {
	cw := csv.NewWriter(w)

	if err := cw.Write(linkCSVHeader); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, l := range links {
		rec := []string{
			l.CRMID,
			strconv.FormatInt(l.SiteID, 10),
		}
		if err := cw.Write(rec); err != nil {
			return fmt.Errorf("write csv record %s: %w", l.ID, err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}
