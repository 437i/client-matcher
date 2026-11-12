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
//
//	@Summary		Создать связь CRM и сайта
//	@Description	Создаёт связь между клиентом CRM и клиентом сайта после проверки совпадения фамилии и даты рождения.
//	@Tags			links
//	@Accept			json
//	@Produce		json
//	@Param			request	body		handlers.CreateRequest	true	"Пара идентификаторов"
//	@Success		201	{object}	handlers.LinkResponse
//	@Failure		400	{object}	handlers.ErrorResponse
//	@Failure		404	{object}	handlers.ErrorResponse
//	@Failure		409	{object}	handlers.ErrorResponse
//	@Failure		422	{object}	handlers.ErrorResponse
//	@Failure		499	{object}	handlers.ErrorResponse	"Запрос отменён клиентом (нестандартный код 499)"
//	@Failure		500	{object}	handlers.ErrorResponse
//	@Failure		502	{object}	handlers.ErrorResponse	"Источник вернул некорректные данные клиента"
//	@Failure		504	{object}	handlers.ErrorResponse	"Превышено время ожидания"
//	@Router			/api/v1/links [post]
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
//
//	@Summary		Список связей
//	@Description	Возвращает все сохранённые связи между клиентами CRM и сайта.
//	@Tags			links
//	@Produce		json
//	@Success		200	{array}	handlers.LinkResponse
//	@Failure		499	{object}	handlers.ErrorResponse	"Запрос отменён клиентом (нестандартный код 499)"
//	@Failure		500	{object}	handlers.ErrorResponse
//	@Failure		504	{object}	handlers.ErrorResponse	"Превышено время ожидания"
//	@Router			/api/v1/links [get]
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
//
// Порядок аннотаций важен: swag применяет @Produce только к ответам,
// объявленным после него. Поэтому @Produce стоит после @Failure — ошибки
// получают application/json, а успешный ответ — text/csv.
//
//	@Summary		Выгрузка связей в CSV
//	@Description	Отдаёт файл CSV с колонками crm_id, site_id.
//	@Tags			links
//	@Failure		499	{object}	handlers.ErrorResponse	"Запрос отменён клиентом (нестандартный код 499)"
//	@Failure		500	{object}	handlers.ErrorResponse	"Внутренняя ошибка"
//	@Failure		504	{object}	handlers.ErrorResponse	"Превышено время ожидания"
//	@Produce		text/csv
//	@Success		200	{string}	string	"crm_id,site_id"
//	@Router			/api/v1/links/export.csv [get]
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
