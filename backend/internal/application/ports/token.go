package ports

import (
	"context"
	"errors"
	"github.com/437i/client-matcher/backend/internal/domain/client"
	"time"
)

const TokenVersion = 2
const TokenTTL = 5 * time.Minute

var ErrInvalidAssessmentToken = errors.New("invalid assessment token")
var ErrAssessmentExpired = errors.New("assessment expired")

type Clock interface{ Now() time.Time }
type TokenClaims struct {
	CRMID        string    `json:"crm_id"`
	SiteID       int64     `json:"site_id"`
	Fingerprint  [32]byte  `json:"fingerprint"`
	IssuedAt     time.Time `json:"issued_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenVersion int       `json:"token_version"`
	RuleVersion  int       `json:"rule_version"`
}
type AssessmentToken interface {
	Fingerprint(client.CRMClient, client.SiteClient) [32]byte
	Issue(context.Context, TokenClaims) (string, error)
	Verify(context.Context, string, string, int64) (TokenClaims, error)
}
