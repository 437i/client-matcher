package handlers

import (
	"net/http"

	"github.com/437i/client-matcher/backend/internal/application/services"
	"github.com/437i/client-matcher/backend/internal/domain/matcher"
	"github.com/gin-gonic/gin"
)

type AssessmentHandler struct{ service *services.ClientLinkService }

func NewAssessmentHandler(service *services.ClientLinkService) *AssessmentHandler {
	return &AssessmentHandler{service: service}
}

// Assess оценивает пару без сохранения связи.
//
// @Summary Оценить пару CRM и сайта
// @Description Возвращает ready, requires_confirmation или blocked. Токен действует пять минут. При blocked проверки пусты, токен и срок действия равны null.
// @Tags links
// @Accept json
// @Produce json
// @Param request body handlers.AssessmentRequest true "Идентификаторы пары"
// @Success 200 {object} handlers.AssessmentResponse
// @Failure 400 {object} handlers.ErrorResponse "invalid_body для отсутствующих полей или JSON-числа, invalid_id для некорректной строки ID"
// @Failure 413 {object} handlers.ErrorResponse
// @Failure 499 {object} handlers.ErrorResponse "Запрос отменён клиентом"
// @Failure 500 {object} handlers.ErrorResponse
// @Failure 502 {object} handlers.ErrorResponse
// @Failure 504 {object} handlers.ErrorResponse
// @Router /api/v1/link-assessments [post]
func (h *AssessmentHandler) Assess(c *gin.Context) {
	var req AssessmentRequest
	if err := bindLinkJSON(c, &req); err != nil {
		return
	}
	if req.CRMID == "" || req.SiteID == "" {
		respondBadRequest(c, "invalid_body", "Укажите crm_id и site_id")
		return
	}
	siteID, ok := parseSiteID(c, req.SiteID)
	if !ok {
		return
	}
	a, err := h.service.Assess(c.Request.Context(), req.CRMID, siteID)
	if err != nil {
		respondError(c, err)
		return
	}
	checks := make([]AssessmentCheckResponse, 0, len(a.Checks))
	for _, check := range a.Checks {
		if check.Field == matcher.FieldLastName && a.FullNameMismatch != nil {
			check.Code = matcher.CodeMismatch
			check.CRMValue = &a.FullNameMismatch.CRMValue
			check.SiteValue = &a.FullNameMismatch.SiteValue
		}
		checks = append(checks, AssessmentCheckResponse{Field: string(check.Field), Code: string(check.Code), CRMValue: check.CRMValue, SiteValue: check.SiteValue})
	}
	c.JSON(http.StatusOK, AssessmentResponse{Status: string(a.Status), Checks: checks, ConfirmationRequired: a.ConfirmationRequired, Token: a.Token, ExpiresAt: a.ExpiresAt, BlockedReasons: a.BlockedReasons})
}
