package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupRoutes(appServer *AppServer) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(errorHandlingMiddleware())
	router.Use(corsMiddleware())

	router.GET("/health", healthHandler)

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return appServer.mcpServer
		},
		&mcp.StreamableHTTPOptions{
			JSONResponse: true,
		},
	)
	router.Any("/mcp", gin.WrapH(mcpHandler))
	router.Any("/mcp/*path", gin.WrapH(mcpHandler))

	api := router.Group("/api/v1")
	{
		api.GET("/login/status", appServer.checkLoginStatusHandler)
		api.GET("/login/qrcode", appServer.getLoginQrcodeHandler)
		api.DELETE("/login/cookies", appServer.deleteCookiesHandler)
		api.GET("/search", appServer.searchItemsHandler)
		api.POST("/search", appServer.searchItemsHandler)
		api.GET("/im/conversations", appServer.listConversationsHandler)
		api.GET("/im/messages", appServer.getMessagesHandler)
		api.POST("/im/messages", appServer.getMessagesHandler)
		api.POST("/im/send", appServer.sendMessageHandler)
		api.POST("/publish/item", appServer.publishItemHandler)
		api.GET("/orders/list", appServer.listOrdersHandler)
		api.POST("/orders/remind_ship", appServer.remindShipHandler)
		api.POST("/orders/ship", appServer.shipOrderHandler)
	}

	return router
}
