package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/swaggest/swgui/v5emb"

	"github.com/437i/client-matcher/backend/openapi"
)

// Маршруты документации вынесены из /api/v1: они описывают API, но сами его
// частью не являются. Спецификация и Swagger UI отдаются локально, без CDN.
const (
	specJSONPath = "/openapi.json"
	specYAMLPath = "/openapi.yaml"
	docsBasePath = "/docs/"
)

func registerDocs(engine *gin.Engine) {
	engine.GET(specJSONPath, func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", openapi.JSON)
	})
	engine.GET(specYAMLPath, func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", openapi.YAML)
	})

	ui := v5emb.NewHandler("Client Matcher API", specJSONPath, docsBasePath)
	engine.GET("/docs", gin.WrapH(ui))
	engine.GET(docsBasePath+"*any", gin.WrapH(ui))
	engine.HEAD("/docs", gin.WrapH(ui))
	engine.HEAD(docsBasePath+"*any", gin.WrapH(ui))
}
