package ports

import (
	"context"

	"github.com/437i/client-matcher/backend/internal/domain/client"
)

type SiteReader interface {
	// GetByID возвращает client.ErrSiteNotFound (можно обёрнутую), если записи нет.
	GetByID(ctx context.Context, id int64) (client.SiteClient, error)
}
