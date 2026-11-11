package ports

import (
	"context"

	"github.com/437i/client-matcher/backend/internal/domain/client"
)

type CRMReader interface {
	// Search ищет клиентов по произвольному имени («Иванов», «Иванов Ив», «Иван Иванов»).
	// Возвращает не более limit записей в стабильном порядке; пустой результат — не ошибка.
	Search(ctx context.Context, query string, limit int) ([]client.CRMClient, error)

	// GetByID возвращает client.ErrCRMNotFound (можно обёрнутую), если записи нет.
	GetByID(ctx context.Context, id string) (client.CRMClient, error)
}
