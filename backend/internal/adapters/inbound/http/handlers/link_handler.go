package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/437i/client-matcher/backend/internal/application/services"
)

type LinkHandler struct {
	linkService *services.ClientLinkService
}

func NewLinkHandler(svc *services.ClientLinkService) *LinkHandler {
	return &LinkHandler{linkService: svc}
}

// CreateLink сохраняет связь: POST /api/v1/links {"crm_id": "...", "site_id": "..."}
func (h *LinkHandler) CreateLink(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "invalid_body", "Укажите crm_id и site_id")
		return
	}

	result, err := h.linkService.CreateLink(c.Request.Context(), req.CRMID, req.SiteID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, newLinkResponse(result))
}

// GetList возвращает все сохранённые связи: GET /api/v1/links
func (h *LinkHandler) GetList(c *gin.Context) {
	result, err := h.linkService.GetList(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, newLinkResponses(result))
}

// ExportCSV выгружает все связи в CSV с колонками crm_id, site_id:
// GET /api/v1/links/export.csv
func (h *LinkHandler) ExportCSV(c *gin.Context) {
	links, err := h.linkService.GetList(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}

	filename := fmt.Sprintf("client_links_%s.csv", time.Now().UTC().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	if err := writeLinksCSV(c.Writer, links); err != nil {
		// заголовки уже отправлены — статус не изменить, остаётся залогировать
		slog.ErrorContext(c.Request.Context(), "export links csv", "error", err)
	}
}
