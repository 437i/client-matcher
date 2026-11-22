package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/application/services"
	"github.com/437i/client-matcher/backend/internal/domain/client"
	"github.com/437i/client-matcher/backend/internal/domain/link"
)

// ErrorResponse — единый формат ошибки. Code стабилен и предназначен для
// фронтенда, Message — для показа пользователю.
type ErrorResponse struct {
	Code    string `json:"code" example:"crm_client_not_found"`
	Message string `json:"message" example:"Клиент не найден в CRM"`
}

// 499 — нестандартный, но общепринятый статус «клиент закрыл соединение».
const statusClientClosedRequest = 499

type apiError struct {
	status  int
	code    string
	message string
}

func mapError(err error) apiError {
	switch {
	case errors.Is(err, services.ErrQueryEmpty):
		return apiError{http.StatusBadRequest, "query_empty", "Введите имя клиента"}
	case errors.Is(err, services.ErrInvalidID):
		return apiError{400, "invalid_id", "Укажите оба ID"}
	case errors.Is(err, ports.ErrAssessmentExpired):
		return apiError{409, "assessment_expired", "Срок проверки истёк. Повторите проверку пары"}
	case errors.Is(err, services.ErrAssessmentStale):
		return apiError{409, "assessment_stale", "Данные изменились. Повторите проверку пары"}
	case errors.Is(err, services.ErrConfirmationRequired):
		return apiError{http.StatusUnprocessableEntity, "confirmation_required", "Подтвердите расхождения и укажите причину"}
	case errors.Is(err, services.ErrInvalidConfirmation):
		return apiError{http.StatusBadRequest, "invalid_confirmation", "Проверьте подтверждение и причину (от 1 до 1000 символов)"}
	case errors.Is(err, ports.ErrInvalidAssessmentToken):
		return apiError{http.StatusBadRequest, "invalid_assessment_token", "Проверка пары устарела. Повторите проверку"}
	case errors.Is(err, client.ErrInvalidSourceData):
		return apiError{http.StatusBadGateway, "invalid_source_client_data", "Источник вернул некорректные данные клиента"}
	case errors.Is(err, client.ErrCRMNotFound):
		return apiError{http.StatusNotFound, "crm_client_not_found", "Клиент не найден в CRM"}
	case errors.Is(err, client.ErrSiteNotFound):
		return apiError{http.StatusNotFound, "site_client_not_found", "Клиент с таким ID не найден на сайте"}
	case errors.Is(err, link.ErrNotFound):
		return apiError{http.StatusNotFound, "link_not_found", "Связь не найдена"}
	case errors.Is(err, link.ErrCRMAlreadyLinked):
		return apiError{http.StatusConflict, "crm_already_linked", "Этот клиент CRM уже связан с ID сайта"}
	case errors.Is(err, link.ErrSiteAlreadyLinked):
		return apiError{http.StatusConflict, "site_already_linked", "Этот ID сайта уже связан с клиентом CRM"}
	case errors.Is(err, context.Canceled):
		return apiError{statusClientClosedRequest, "request_canceled", "Запрос отменён"}
	case errors.Is(err, context.DeadlineExceeded):
		return apiError{http.StatusGatewayTimeout, "timeout", "Превышено время ожидания"}
	default:
		return apiError{http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера"}
	}
}

// В журнал попадает код ошибки без данных источников и запроса.
func respondError(c *gin.Context, err error) {
	apiErr := mapError(err)
	if apiErr.status >= http.StatusInternalServerError {
		slog.ErrorContext(c.Request.Context(), "request failed",
			"method", c.Request.Method, "path", c.FullPath(), "code", apiErr.code)
	}
	c.JSON(apiErr.status, ErrorResponse{Code: apiErr.code, Message: apiErr.message})
}

func respondBadRequest(c *gin.Context, code, message string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{Code: code, Message: message})
}
