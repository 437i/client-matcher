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
