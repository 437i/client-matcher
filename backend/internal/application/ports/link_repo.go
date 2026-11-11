package ports

import (
	"context"

	"github.com/437i/client-matcher/backend/internal/domain/link"
)

type ClientLinkRepo interface {
	Create(ctx context.Context, data link.ClientLink) (link.ClientLink, error)
	List(ctx context.Context) ([]link.ClientLink, error)
}
