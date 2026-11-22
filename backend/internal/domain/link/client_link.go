package link

import (
	"time"

	"github.com/google/uuid"
)

type ClientLink struct {
	ID        uuid.UUID
	CRMID     string
	SiteID    int64
	CreatedAt time.Time
}

type Decision struct {
	CRMID        string
	SiteID       int64
	RuleVersion  int
	Confirmation *Confirmation
}
type Confirmation struct {
	Reason string
	Codes  []string
	At     time.Time
}
