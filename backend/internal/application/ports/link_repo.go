package ports

import (
	"context"

	"github.com/437i/client-matcher/backend/internal/domain/link"
)

type ClientLinkRepo interface {
	LookupBySourceIDs(ctx context.Context, crmID string, siteID int64) (crmOccupied, siteOccupied bool, err error)
	Create(ctx context.Context, data link.Decision) (link.ClientLink, error)
	List(ctx context.Context) ([]link.ClientLink, error)
}
