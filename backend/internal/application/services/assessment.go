package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/client"
	"github.com/437i/client-matcher/backend/internal/domain/matcher"
)

type AssessmentResult struct {
	matcher.Assessment
	BlockedReasons []string
	Token          *string
	ExpiresAt      *time.Time
}

func (s *ClientLinkService) Assess(ctx context.Context, crmID string, siteID int64) (AssessmentResult, error) {
	crmID = strings.TrimSpace(crmID)
	if crmID == "" {
		return AssessmentResult{}, ErrInvalidID
	}
	crm, crmErr := s.readCRM(ctx, crmID)
	if crmErr != nil && !errors.Is(crmErr, client.ErrCRMNotFound) {
		return AssessmentResult{}, crmErr
	}
	site, siteErr := s.readSite(ctx, siteID)
	if siteErr != nil && !errors.Is(siteErr, client.ErrSiteNotFound) {
		return AssessmentResult{}, siteErr
	}
	reasons := []string{}
	if crmErr != nil {
		reasons = append(reasons, "crm_not_found")
	}
	if siteErr != nil {
		reasons = append(reasons, "site_not_found")
	}
	if len(reasons) == 0 {
		crmOccupied, siteOccupied, err := s.linkRepo.LookupBySourceIDs(ctx, crmID, siteID)
		if err != nil {
			return AssessmentResult{}, fmt.Errorf("lookup links: %w", err)
		}
		if crmOccupied {
			reasons = append(reasons, "crm_already_linked")
		}
		if siteOccupied {
			reasons = append(reasons, "site_already_linked")
		}
	}
	if len(reasons) > 0 {
		return AssessmentResult{Assessment: matcher.Assessment{Status: matcher.StatusBlocked}, BlockedReasons: reasons}, nil
	}
	now := s.clock.Now().UTC()
	expires := now.Add(ports.TokenTTL)
	token, err := s.tokens.Issue(ctx, ports.TokenClaims{CRMID: crmID, SiteID: siteID, Fingerprint: s.tokens.Fingerprint(crm, site), IssuedAt: now, ExpiresAt: expires, TokenVersion: ports.TokenVersion, RuleVersion: matcher.RuleVersion})
	if err != nil {
		return AssessmentResult{}, fmt.Errorf("issue assessment: %w", err)
	}
	return AssessmentResult{Assessment: matcher.Assess(crm, site), BlockedReasons: reasons, Token: &token, ExpiresAt: &expires}, nil
}

func (s *ClientLinkService) readCRM(ctx context.Context, id string) (client.CRMClient, error) {
	record, err := s.crmRepo.GetByID(ctx, id)
	if errors.Is(err, client.ErrInvalidCRMID) {
		return record, ErrInvalidID
	}
	if err != nil {
		return record, fmt.Errorf("read CRM record: %w", err)
	}
	if record.Birthday != nil && !record.Birthday.IsValid() {
		return record, client.ErrInvalidSourceData
	}
	return record, nil
}

func (s *ClientLinkService) readSite(ctx context.Context, id int64) (client.SiteClient, error) {
	record, err := s.siteRepo.GetByID(ctx, id)
	if err != nil {
		return record, fmt.Errorf("read site record: %w", err)
	}
	if record.Birthday != nil && !record.Birthday.IsValid() {
		return record, client.ErrInvalidSourceData
	}
	return record, nil
}
