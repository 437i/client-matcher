package services

import (
	"context"
	"errors"
	"strings"

	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/client"
)

// searchLimit ограничивает выдачу поиска: редактору нужен короткий список,
// а реальный CRM API не должен отдавать тысячи записей.
const searchLimit = 50

var ErrQueryEmpty = errors.New("search query is empty")

type ClientSearchService struct {
	crmRepo  ports.CRMReader
	siteRepo ports.SiteReader
}

func NewClientSearchService(crmRepo ports.CRMReader, siteRepo ports.SiteReader) *ClientSearchService {
	return &ClientSearchService{
		crmRepo:  crmRepo,
		siteRepo: siteRepo,
	}
}

func (s *ClientSearchService) SearchClients(ctx context.Context, query string) ([]client.CRMClient, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrQueryEmpty
	}

	return s.crmRepo.Search(ctx, query, searchLimit)
}

// GetSiteClient нужен интерфейсу, чтобы показать редактору, кому принадлежит
// введённый им ID сайта, до сохранения связи.
func (s *ClientSearchService) GetSiteClient(ctx context.Context, id string) (client.SiteClient, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return client.SiteClient{}, ErrIDEmpty
	}

	return s.siteRepo.GetByID(ctx, id)
}
