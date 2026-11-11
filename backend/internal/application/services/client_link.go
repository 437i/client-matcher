package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/link"
	"github.com/437i/client-matcher/backend/internal/domain/matcher"
)

var ErrIDEmpty = errors.New("id is empty")

type ClientLinkService struct {
	crmRepo  ports.CRMReader
	siteRepo ports.SiteReader
	linkRepo ports.ClientLinkRepo
}

func NewClientLinkService(crmRepo ports.CRMReader, siteRepo ports.SiteReader, linkRepo ports.ClientLinkRepo) *ClientLinkService {
	return &ClientLinkService{
		crmRepo:  crmRepo,
		siteRepo: siteRepo,
		linkRepo: linkRepo,
	}
}

// CreateLink проверяет, что обе записи существуют и описывают одного человека,
// после чего сохраняет пару ID. Уникальность пары гарантирует БД (а не
// предварительный SELECT), поэтому гонки двух редакторов невозможны.
func (s *ClientLinkService) CreateLink(ctx context.Context, crmID, siteID string) (link.ClientLink, error) {
	crmID, siteID = strings.TrimSpace(crmID), strings.TrimSpace(siteID)
	if crmID == "" || siteID == "" {
		return link.ClientLink{}, ErrIDEmpty
	}

	crmData, err := s.crmRepo.GetByID(ctx, crmID)
	if err != nil {
		return link.ClientLink{}, fmt.Errorf("get crm client %q: %w", crmID, err)
	}

	siteData, err := s.siteRepo.GetByID(ctx, siteID)
	if err != nil {
		return link.ClientLink{}, fmt.Errorf("get site client %q: %w", siteID, err)
	}

	if err := matcher.MatchClients(crmData, siteData); err != nil {
		return link.ClientLink{}, fmt.Errorf("crm %q and site %q don't match: %w", crmID, siteID, err)
	}

	result, err := s.linkRepo.Create(ctx, link.ClientLink{CRMID: crmID, SiteID: siteID})
	if err != nil {
		return link.ClientLink{}, fmt.Errorf("create link %q-%q: %w", crmID, siteID, err)
	}

	return result, nil
}

func (s *ClientLinkService) GetList(ctx context.Context) ([]link.ClientLink, error) {
	return s.linkRepo.List(ctx)
}
