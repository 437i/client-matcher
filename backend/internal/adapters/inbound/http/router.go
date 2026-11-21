package http

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/437i/client-matcher/backend/internal/adapters/inbound/http/handlers"
)

func NewRouter(
	clientHandler *handlers.ClientHandler,
	linkHandler *handlers.LinkHandler,
	assessmentHandler *handlers.AssessmentHandler,
	healthHandler *handlers.HealthHandler,
) (*gin.Engine, error) {
	engine := gin.New()
	// Поисковая строка содержит ФИО. Не записываем query string в access-логи.
	engine.Use(
		gin.LoggerWithConfig(gin.LoggerConfig{SkipQueryString: true}),
		gin.Recovery(),
	)

	// Не доверяем X-Forwarded-For от кого попало. За reverse proxy укажите его адрес.
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	engine.GET("/healthz", healthHandler.Health)

	v1 := engine.Group("/api/v1")
	v1.POST("/link-assessments", assessmentHandler.Assess)

	v1.GET("/crm-clients", clientHandler.Search)
	v1.GET("/site-clients/:id", clientHandler.GetSiteClient)

	links := v1.Group("/links")
	{
		links.POST("", linkHandler.CreateLink)
		links.GET("", linkHandler.GetList)
		links.GET("/export.csv", linkHandler.ExportCSV)
	}

	registerDocs(engine)

	return engine, nil
}
