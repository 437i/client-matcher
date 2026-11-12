package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	ping func(ctx context.Context) error
}

func NewHealthHandler(ping func(ctx context.Context) error) *HealthHandler {
	return &HealthHandler{ping: ping}
}

// Health: GET /healthz — для healthcheck в Docker и мониторинга.
//
//	@Summary		Проверка готовности сервиса
//	@Description	Возвращает статус backend и доступности базы данных.
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/healthz [get]
func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
