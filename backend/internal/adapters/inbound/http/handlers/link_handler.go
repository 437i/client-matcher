package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
//	@Description	Повторно получает обе записи. Для ready достаточно двух ID. Для requires_confirmation передайте актуальный assessment_token, confirmed=true и причину длиной 1–1000 символов после обрезки пробелов. Не указывайте ФИО и даты рождения в причине.
//	@Tags			links
//	@Accept			json
//	@Produce		json
//	@Param			request	body		handlers.CreateRequest	true	"Пара идентификаторов"
//	@Success		201	{object}	handlers.LinkResponse
//	@Failure		400	{object}	handlers.ErrorResponse "invalid_body для отсутствующих полей или JSON-числа, invalid_id для некорректной строки ID"
//	@Failure		404	{object}	handlers.ErrorResponse
//	@Failure		409	{object}	handlers.ErrorResponse
//
// @Failure 413 {object} handlers.ErrorResponse "Тело запроса превышает 16 КиБ"
//
//	@Failure		422	{object}	handlers.ErrorResponse
//	@Failure		499	{object}	handlers.ErrorResponse	"Запрос отменён клиентом (нестандартный код 499)"
//	@Failure		500	{object}	handlers.ErrorResponse
//	@Failure		502	{object}	handlers.ErrorResponse	"Источник вернул некорректные данные клиента"
//	@Failure		504	{object}	handlers.ErrorResponse	"Превышено время ожидания"
//	@Router			/api/v1/links [post]
func (h *LinkHandler) CreateLink(c *gin.Context) {
	var req CreateRequest
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
	confirmed := req.Confirmed != nil && *req.Confirmed
	result, err := h.linkService.CreateLink(c.Request.Context(), req.CRMID, siteID, req.AssessmentToken, confirmed, req.Reason)
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

func bindLinkJSON(c *gin.Context, target any) error {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024))
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Code: "request_too_large", Message: "Запрос слишком большой"})
		return err
	}
	if err == nil {
		if bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
			err = errors.New("expected JSON object")
		} else {
			err = json.Unmarshal(body, target)
		}
	}
	if err != nil {
		respondBadRequest(c, "invalid_body", "Укажите crm_id и site_id")
	}
	return err
}
