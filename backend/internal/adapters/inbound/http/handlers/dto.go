package handlers

import (
	"time"

	"github.com/google/uuid"

	"github.com/437i/client-matcher/backend/internal/domain/client"
	"github.com/437i/client-matcher/backend/internal/domain/link"
)

// Слой HTTP не отдаёт доменные структуры напрямую: формат API (snake_case,
// дата строкой) не должен меняться вместе с доменной моделью.

type SearchQuery struct {
	Q string `form:"q" binding:"required"`
}

type CreateRequest struct {
	CRMID  string `json:"crm_id" binding:"required"`
	SiteID string `json:"site_id" binding:"required"`
}

type CRMClientResponse struct {
	ID         string `json:"id"`
	FullName   string `json:"full_name"`
	LastName   string `json:"last_name"`
	FirstName  string `json:"first_name"`
	Patronymic string `json:"patronymic,omitempty"`
	Birthday   string `json:"birthday"` // YYYY-MM-DD
	Clinic     string `json:"clinic"`
}

type SiteClientResponse struct {
	ID        string `json:"id"`
	ShortName string `json:"short_name"`
	LastName  string `json:"last_name"`
	Birthday  string `json:"birthday"` // YYYY-MM-DD
}

type LinkResponse struct {
	ID        uuid.UUID `json:"id"` // UUID связи
	CRMID     string    `json:"crm_id"`
	SiteID    string    `json:"site_id"`
	CreatedAt time.Time `json:"created_at"` // RFC 3339, например 2026-11-11T21:00:00Z
}

func newCRMClientResponse(c client.CRMClient) CRMClientResponse {
	return CRMClientResponse{
		ID:         c.ID,
		FullName:   c.FullName(),
		LastName:   c.LastName,
		FirstName:  c.FirstName,
		Patronymic: c.Patronymic,
		Birthday:   c.Birthday.String(),
		Clinic:     c.Clinic,
	}
}

func newSiteClientResponse(c client.SiteClient) SiteClientResponse {
	return SiteClientResponse{
		ID:        c.ID,
		ShortName: c.ShortName,
		LastName:  c.LastName,
		Birthday:  c.Birthday.String(),
	}
}

func newLinkResponse(l link.ClientLink) LinkResponse {
	return LinkResponse{ID: l.ID, CRMID: l.CRMID, SiteID: l.SiteID, CreatedAt: l.CreatedAt}
}

// make(..., 0, n): пустой список должен сериализоваться как [], а не null.
func newCRMClientResponses(list []client.CRMClient) []CRMClientResponse {
	out := make([]CRMClientResponse, 0, len(list))
	for _, c := range list {
		out = append(out, newCRMClientResponse(c))
	}
	return out
}

func newLinkResponses(list []link.ClientLink) []LinkResponse {
	out := make([]LinkResponse, 0, len(list))
	for _, l := range list {
		out = append(out, newLinkResponse(l))
	}
	return out
}
