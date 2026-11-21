package outbound

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"

	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/client"
)

const maxTokenBytes = 4096

var encoding = base64.RawURLEncoding.Strict()

type HMACToken struct {
	signingKey, fingerprintKey [32]byte
	clock                      ports.Clock
}

func NewHMACToken(key [32]byte, clock ports.Clock) *HMACToken {
	return &HMACToken{digest(key, []byte("assessment/signature/v1")), digest(key, []byte("assessment/fingerprint/v1")), clock}
}

func GenerateTokenKey() ([32]byte, error) {
	var key [32]byte
	_, err := rand.Read(key[:])
	return key, err
}

func digest(key [32]byte, data []byte) [32]byte {
	mac := hmac.New(sha256.New, key[:])
	mac.Write(data)
	return [32]byte(mac.Sum(nil))
}

func sourceBytes(crm client.CRMClient, site client.SiteClient) []byte {
	var data []byte
	add := func(s string) { data = binary.BigEndian.AppendUint64(data, uint64(len(s))); data = append(data, s...) }
	field := func(name, value string) { add(name); data = append(data, 1); add(value) }
	date := func(name string, d *client.Date) {
		if d == nil {
			add(name)
			data = append(data, 0)
			return
		}
		field(name, d.String())
	}
	field("crm.id", crm.ID)
	field("crm.last_name", crm.LastName)
	field("crm.first_name", crm.FirstName)
	field("crm.patronymic", crm.Patronymic)
	field("crm.unstructured_name", crm.UnstructuredName)
	date("crm.birthday", crm.Birthday)
	field("site.id", site.ID)
	field("site.last_name", site.LastName)
	field("site.first_name", site.ShortName)
	field("site.unstructured_name", site.UnstructuredName)
	date("site.birthday", site.Birthday)
	return data
}

func (h *HMACToken) Fingerprint(crm client.CRMClient, site client.SiteClient) [32]byte {
	return digest(h.fingerprintKey, sourceBytes(crm, site))
}

func (h *HMACToken) Issue(ctx context.Context, claims ports.TokenClaims) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	sig := digest(h.signingKey, payload)
	token := encoding.EncodeToString(payload) + "." + encoding.EncodeToString(sig[:])
	if len(token) > maxTokenBytes {
		return "", ports.ErrInvalidAssessmentToken
	}
	return token, nil
}

func (h *HMACToken) Verify(ctx context.Context, token, crmID, siteID string) (ports.TokenClaims, error) {
	if err := ctx.Err(); err != nil {
		return ports.TokenClaims{}, err
	}
	invalid := ports.ErrInvalidAssessmentToken
	if len(token) > maxTokenBytes || strings.ContainsAny(token, "\r\n") {
		return ports.TokenClaims{}, invalid
	}
	raw, sig, ok := strings.Cut(token, ".")
	if !ok {
		return ports.TokenClaims{}, invalid
	}
	payload, err := encoding.DecodeString(raw)
	if err != nil {
		return ports.TokenClaims{}, invalid
	}
	signature, err := encoding.DecodeString(sig)
	want := digest(h.signingKey, payload)
	if err != nil || !hmac.Equal(signature, want[:]) {
		return ports.TokenClaims{}, invalid
	}
	var claims ports.TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.TokenVersion != ports.TokenVersion || claims.RuleVersion <= 0 || claims.IssuedAt.IsZero() || !claims.ExpiresAt.Equal(claims.IssuedAt.Add(ports.TokenTTL)) {
		return ports.TokenClaims{}, invalid
	}
	if !h.clock.Now().Before(claims.ExpiresAt) {
		return ports.TokenClaims{}, ports.ErrAssessmentExpired
	}
	if claims.CRMID != crmID || claims.SiteID != siteID {
		return ports.TokenClaims{}, invalid
	}
	return claims, nil
}
