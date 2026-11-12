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
//
//	@Summary		Поиск клиентов в CRM
//	@Description	Ищет клиентов CRM по имени. Возвращает не более 50 записей.
//	@Tags			crm
//	@Produce		json
//	@Param			q	query		string	true	"Фрагмент имени или фамилии"	example(иванов)
//	@Success		200	{array}		handlers.CRMClientResponse
//	@Failure		400	{object}	handlers.ErrorResponse
//	@Failure		499	{object}	handlers.ErrorResponse	"Запрос отменён клиентом (нестандартный код 499)"
//	@Failure		500	{object}	handlers.ErrorResponse
//	@Failure		504	{object}	handlers.ErrorResponse	"Превышено время ожидания"
//	@Router			/api/v1/crm-clients [get]
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
//
//	@Summary		Клиент сайта по ID
//	@Description	Возвращает клиента сайта по его ID, чтобы сверить данные перед сохранением связи.
//	@Tags			site
//	@Produce		json
//	@Param			id	path		string	true	"ID клиента на сайте"	example(3586)
//	@Success		200	{object}	handlers.SiteClientResponse
//	@Failure		400	{object}	handlers.ErrorResponse
//	@Failure		404	{object}	handlers.ErrorResponse
//	@Failure		499	{object}	handlers.ErrorResponse	"Запрос отменён клиентом (нестандартный код 499)"
//	@Failure		500	{object}	handlers.ErrorResponse
//	@Failure		504	{object}	handlers.ErrorResponse	"Превышено время ожидания"
//	@Router			/api/v1/site-clients/{id} [get]
func (h *ClientHandler) GetSiteClient(c *gin.Context) {
	cl, err := h.clientService.GetSiteClient(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, newSiteClientResponse(cl))
}
