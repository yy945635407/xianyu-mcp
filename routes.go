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
		api.GET("/im/events", appServer.pullIMEventsHandler)
		api.POST("/im/events", appServer.pullIMEventsHandler)
		api.GET("/im/events/wait", appServer.waitIMEventsHandler)
		api.POST("/im/events/wait", appServer.waitIMEventsHandler)
		api.GET("/im/session/state", appServer.getIMSessionStateHandler)
		api.GET("/im/session/states", appServer.listIMSessionStatesHandler)
		api.POST("/im/session/state", appServer.setIMSessionStateHandler)
		api.POST("/im/session/mark_read", appServer.markIMSessionReadHandler)
		api.POST("/im/send", appServer.sendMessageHandler)
		api.GET("/im/kb/list", appServer.listIMKnowledgeHandler)
		api.POST("/im/kb/upsert", appServer.upsertIMKnowledgeHandler)
		api.POST("/im/kb/delete", appServer.deleteIMKnowledgeHandler)
		api.POST("/im/kb/match", appServer.matchIMKnowledgeHandler)
		api.POST("/publish/item", appServer.publishItemHandler)
		api.GET("/orders/list", appServer.listOrdersHandler)
		api.POST("/orders/remind_ship", appServer.remindShipHandler)
		api.POST("/orders/ship", appServer.shipOrderHandler)
		api.GET("/collections/list", appServer.listCollectionsHandler)
		api.POST("/collections/cancel", appServer.cancelFavoriteHandler)
		api.POST("/collections/groups/manage", appServer.manageCollectionGroupHandler)
		api.GET("/my/items", appServer.listMyItemsHandler)
		api.POST("/my/items/edit", appServer.editMyItemHandler)
		api.POST("/my/items/shelf", appServer.shelfMyItemHandler)
		api.POST("/my/items/delete", appServer.deleteMyItemHandler)
		api.GET("/item/detail", appServer.getItemDetailHandler)
		api.POST("/item/favorite", appServer.favoriteItemHandler)
		api.POST("/item/chat", appServer.chatItemHandler)
		api.POST("/item/buy", appServer.buyItemHandler)
		api.GET("/account/security", appServer.getAccountSecurityHandler)
		api.GET("/community/feed", appServer.getCommunityFeedHandler)
		api.POST("/community/interact", appServer.interactCommunityHandler)
		api.GET("/customer/service", appServer.getCustomerServiceHandler)
		api.POST("/customer/open", appServer.openCustomerServiceHandler)
		api.POST("/orders/ship_with_logistics", appServer.shipWithLogisticsHandler)
		api.POST("/orders/confirm_receipt", appServer.confirmReceiptHandler)
		api.POST("/orders/review", appServer.reviewOrderHandler)
		api.POST("/orders/refund", appServer.refundActionHandler)
	}

	return router
}
