package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/link"
	"github.com/437i/client-matcher/backend/internal/domain/matcher"
)

var (
	ErrIDEmpty              = errors.New("id is empty")
	ErrInvalidID            = errors.New("invalid id")
	ErrConfirmationRequired = errors.New("confirmation required")
	ErrInvalidConfirmation  = errors.New("invalid confirmation")
	ErrAssessmentStale      = errors.New("assessment stale")
)

type ClientLinkService struct {
	crmRepo  ports.CRMReader
	siteRepo ports.SiteReader
	linkRepo ports.ClientLinkRepo
	tokens   ports.AssessmentToken
	clock    ports.Clock
}

func NewClientLinkService(crm ports.CRMReader, site ports.SiteReader, links ports.ClientLinkRepo, tokens ports.AssessmentToken, clock ports.Clock) *ClientLinkService {
	return &ClientLinkService{crmRepo: crm, siteRepo: site, linkRepo: links, tokens: tokens, clock: clock}
}

func (s *ClientLinkService) CreateLink(ctx context.Context, crmID, siteID string, token *string, confirmed bool, reason string) (link.ClientLink, error) {
	crmID, siteID = strings.TrimSpace(crmID), strings.TrimSpace(siteID)
	if crmID == "" || siteID == "" {
		return link.ClientLink{}, ErrInvalidID
	}
	var claims ports.TokenClaims
	if token != nil {
		var err error
		claims, err = s.tokens.Verify(ctx, *token, crmID, siteID)
		if err != nil {
			return link.ClientLink{}, err
		}
	}
	crm, err := s.readCRM(ctx, crmID)
	if err != nil {
		return link.ClientLink{}, err
	}
	site, err := s.readSite(ctx, siteID)
	if err != nil {
		return link.ClientLink{}, err
	}
	if token != nil && (claims.RuleVersion != matcher.RuleVersion || claims.Fingerprint != s.tokens.Fingerprint(crm, site)) {
		return link.ClientLink{}, ErrAssessmentStale
	}
	crmOccupied, siteOccupied, err := s.linkRepo.LookupBySourceIDs(ctx, crmID, siteID)
	if err != nil {
		return link.ClientLink{}, fmt.Errorf("lookup links: %w", err)
	}
	if crmOccupied {
		return link.ClientLink{}, link.ErrCRMAlreadyLinked
	}
	if siteOccupied {
		return link.ClientLink{}, link.ErrSiteAlreadyLinked
	}
	assessment := matcher.Assess(crm, site)
	reason = strings.TrimSpace(reason)
	decision := link.Decision{CRMID: crmID, SiteID: siteID, RuleVersion: matcher.RuleVersion}
	if assessment.ConfirmationRequired {
		if token == nil || !confirmed {
			return link.ClientLink{}, ErrConfirmationRequired
		}
		if n := utf8.RuneCountInString(reason); n == 0 || n > 1000 || strings.ContainsRune(reason, 0) {
			return link.ClientLink{}, ErrInvalidConfirmation
		}
		now := s.clock.Now().UTC()
		decision.Confirmation = &link.Confirmation{Reason: reason, Codes: assessment.ConfirmationCodes(), At: now}
	} else if confirmed || reason != "" {
		return link.ClientLink{}, ErrInvalidConfirmation
	}
	if err := ctx.Err(); err != nil {
		return link.ClientLink{}, err
	}
	return s.linkRepo.Create(ctx, decision)
}

func (s *ClientLinkService) GetList(ctx context.Context) ([]link.ClientLink, error) {
	return s.linkRepo.List(ctx)
}
