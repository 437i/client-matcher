package link

import (
	"time"

	"github.com/google/uuid"
)

type ClientLink struct {
	ID        uuid.UUID
	CRMID     string
	SiteID    string
	CreatedAt time.Time
}

type Decision struct {
	CRMID, SiteID string
	RuleVersion   int
	Confirmation  *Confirmation
}
type Confirmation struct {
	Reason string
	Codes  []string
	At     time.Time
}
