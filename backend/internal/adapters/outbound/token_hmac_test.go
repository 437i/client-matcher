package outbound

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/client"
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

func TestAssessmentToken(t *testing.T) {
	ctx := context.Background()
	clock := &testClock{time.Unix(1000, 0).UTC()}
	key := [32]byte{1}
	codec := NewHMACToken(key, clock)
	other := NewHMACToken([32]byte{2}, clock)
	crm := client.CRMClient{ID: "crm", FirstName: "Синтетическое Имя", Birthday: &client.Date{Year: 2001, Month: 2, Day: 3}}
	site := client.SiteClient{ID: 9007199254740993}
	fp := codec.Fingerprint(crm, site)
	if codec.signingKey == codec.fingerprintKey || fp == other.Fingerprint(crm, site) || fp != digest(codec.fingerprintKey, sourceBytes(crm, site)) {
		t.Fatal("key separation")
	}
	changed := crm
	changed.Birthday = nil
	if fp == codec.Fingerprint(changed, site) {
		t.Fatal("nil date must differ")
	}
	left, right := crm, crm
	left.FirstName, left.LastName = "аб", "в"
	right.FirstName, right.LastName = "а", "бв"
	if bytes.Equal(sourceBytes(left, site), sourceBytes(right, site)) {
		t.Fatal("ambiguous encoding")
	}
	claims := ports.TokenClaims{CRMID: crm.ID, SiteID: site.ID, Fingerprint: fp, IssuedAt: clock.now, ExpiresAt: clock.now.Add(5 * time.Minute), TokenVersion: ports.TokenVersion, RuleVersion: 2}
	token, err := codec.Issue(ctx, claims)
	if err != nil {
		t.Fatal(err)
	}
	raw, sig, _ := strings.Cut(token, ".")
	payload, err := encoding.DecodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte(crm.FirstName)) || bytes.Contains(payload, []byte(crm.Birthday.String())) {
		t.Fatal("source fields in claims")
	}
	got, err := codec.Verify(ctx, token, crm.ID, site.ID)
	if err != nil || got.SiteID != site.ID || got.Fingerprint != fp || got.RuleVersion != 2 {
		t.Fatalf("verify: %v", err)
	}
	for _, tc := range []struct {
		name, token, crm string
		site             int64
		codec            *HMACToken
	}{
		{"signature", raw + ".A" + sig, crm.ID, site.ID, codec},
		{"wrong crm", token, "wrong", site.ID, codec}, {"wrong site", token, crm.ID, site.ID + 1, codec},
		{"other key", token, crm.ID, site.ID, other}, {"size", strings.Repeat("a", 4097), crm.ID, site.ID, codec},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.codec.Verify(ctx, tc.token, tc.crm, tc.site); !errors.Is(err, ports.ErrInvalidAssessmentToken) {
				t.Fatalf("verify: %v", err)
			}
		})
	}

	legacy := bytes.Replace(payload, []byte(`"site_id":9007199254740993`), []byte(`"site_id":"9007199254740993"`), 1)
	legacy = bytes.Replace(legacy, []byte(`"token_version":2`), []byte(`"token_version":1`), 1)
	legacySignature := digest(codec.signingKey, legacy)
	if _, err := codec.Verify(ctx, encoding.EncodeToString(legacy)+"."+encoding.EncodeToString(legacySignature[:]), crm.ID, site.ID); !errors.Is(err, ports.ErrInvalidAssessmentToken) {
		t.Fatalf("legacy token: %v", err)
	}
	claims.TokenVersion++
	unknown, err := codec.Issue(ctx, claims)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = codec.Verify(ctx, unknown, crm.ID, site.ID); !errors.Is(err, ports.ErrInvalidAssessmentToken) {
		t.Fatalf("version: %v", err)
	}
	clock.now = clock.now.Add(5*time.Minute - time.Nanosecond)
	if _, err = codec.Verify(ctx, token, crm.ID, site.ID); err != nil {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(time.Nanosecond)
	if _, err = codec.Verify(ctx, token, crm.ID, site.ID); !errors.Is(err, ports.ErrAssessmentExpired) {
		t.Fatalf("expiry: %v", err)
	}
}
