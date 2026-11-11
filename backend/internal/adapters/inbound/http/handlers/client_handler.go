package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/437i/client-matcher/backend/internal/application/services"
)

type ClientHandler struct {
	clientService *services.ClientSearchService
}

func NewClientHandler(svc *services.ClientSearchService) *ClientHandler {
	return &ClientHandler{clientService: svc}
}

// Search ищет клиентов CRM по имени: GET /api/v1/crm-clients?q=иванов ив
func (h *ClientHandler) Search(c *gin.Context) {
	var q SearchQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		respondBadRequest(c, "invalid_query", "Укажите параметр q")
		return
	}

	clients, err := h.clientService.SearchClients(c.Request.Context(), q.Q)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, newCRMClientResponses(clients))
}

// GetSiteClient возвращает клиента сайта по ID: GET /api/v1/site-clients/:id
func (h *ClientHandler) GetSiteClient(c *gin.Context) {
	cl, err := h.clientService.GetSiteClient(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, newSiteClientResponse(cl))
}
