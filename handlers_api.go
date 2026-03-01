package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/ylyt_bot/xianyu-mcp/cookies"
)

func respondError(c *gin.Context, statusCode int, code, message string, details any) {
	response := ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	}

	logrus.Errorf("%s %s %s %d", c.Request.Method, c.Request.URL.Path, c.GetString("account"), statusCode)
	c.JSON(statusCode, response)
}

func respondSuccess(c *gin.Context, data any, message string) {
	response := SuccessResponse{
		Success: true,
		Data:    data,
		Message: message,
	}

	logrus.Infof("%s %s %s %d", c.Request.Method, c.Request.URL.Path, c.GetString("account"), http.StatusOK)
	c.JSON(http.StatusOK, response)
}

func (s *AppServer) checkLoginStatusHandler(c *gin.Context) {
	status, err := s.xianyuService.CheckLoginStatus(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "STATUS_CHECK_FAILED", "检查登录状态失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, status, "检查登录状态成功")
}

func (s *AppServer) getLoginQrcodeHandler(c *gin.Context) {
	result, err := s.xianyuService.GetLoginQrcode(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LOGIN_QRCODE_FAILED", "获取登录二维码失败", err.Error())
		return
	}

	respondSuccess(c, result, "获取登录二维码成功")
}

func (s *AppServer) deleteCookiesHandler(c *gin.Context) {
	err := s.xianyuService.DeleteCookies(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_COOKIES_FAILED", "删除 cookies 失败", err.Error())
		return
	}

	cookiePath := cookies.GetCookiesFilePath()
	respondSuccess(c, map[string]any{
		"cookie_path": cookiePath,
		"message":     "Cookies 已成功删除，登录状态已重置。",
	}, "删除 cookies 成功")
}

func (s *AppServer) searchItemsHandler(c *gin.Context) {
	var req SearchItemsRequest

	switch c.Request.Method {
	case http.MethodPost:
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
			return
		}
	default:
		req.Keyword = c.Query("keyword")
		if limitStr := c.Query("limit"); limitStr != "" {
			limit, err := strconv.Atoi(limitStr)
			if err != nil {
				respondError(c, http.StatusBadRequest, "INVALID_LIMIT", "limit 必须是整数", err.Error())
				return
			}
			req.Limit = limit
		}
	}

	if req.Keyword == "" {
		respondError(c, http.StatusBadRequest, "MISSING_KEYWORD", "缺少关键词参数", "keyword is required")
		return
	}

	result, err := s.xianyuService.SearchItems(c.Request.Context(), req.Keyword, req.Limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEARCH_ITEMS_FAILED", "搜索商品失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "搜索商品成功")
}

func (s *AppServer) listConversationsHandler(c *gin.Context) {
	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_LIMIT", "limit 必须是整数", err.Error())
			return
		}
		limit = parsed
	}

	result, err := s.xianyuService.ListConversations(c.Request.Context(), limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIST_CONVERSATIONS_FAILED", "读取消息列表失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "读取消息列表成功")
}

func (s *AppServer) getMessagesHandler(c *gin.Context) {
	var req GetMessagesRequest

	switch c.Request.Method {
	case http.MethodPost:
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
			return
		}
	default:
		req.Username = c.Query("username")
		if limitStr := c.Query("limit"); limitStr != "" {
			limit, err := strconv.Atoi(limitStr)
			if err != nil {
				respondError(c, http.StatusBadRequest, "INVALID_LIMIT", "limit 必须是整数", err.Error())
				return
			}
			req.Limit = limit
		}
	}

	if req.Username == "" {
		respondError(c, http.StatusBadRequest, "MISSING_USERNAME", "缺少用户名参数", "username is required")
		return
	}

	result, err := s.xianyuService.GetConversationMessages(c.Request.Context(), req.Username, req.Limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_MESSAGES_FAILED", "查询消息失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "查询消息成功")
}

func (s *AppServer) sendMessageHandler(c *gin.Context) {
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.Username == "" || req.Message == "" {
		respondError(c, http.StatusBadRequest, "MISSING_PARAMS", "缺少参数", "username and message are required")
		return
	}

	result, err := s.xianyuService.SendMessage(c.Request.Context(), req.Username, req.Message, req.Limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEND_MESSAGE_FAILED", "发送消息失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "发送消息成功")
}

func (s *AppServer) publishItemHandler(c *gin.Context) {
	var req PublishItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}

	result, err := s.xianyuService.PublishItem(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PUBLISH_ITEM_FAILED", "发布闲置失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "发布闲置流程执行成功")
}

func (s *AppServer) listOrdersHandler(c *gin.Context) {
	tab := c.Query("tab")
	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_LIMIT", "limit 必须是整数", err.Error())
			return
		}
		limit = parsed
	}

	result, err := s.xianyuService.ListOrders(c.Request.Context(), tab, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIST_ORDERS_FAILED", "查询订单失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "查询订单成功")
}

func (s *AppServer) remindShipHandler(c *gin.Context) {
	var req RemindShipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}

	result, err := s.xianyuService.RemindShip(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "REMIND_SHIP_FAILED", "提醒发货失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "提醒发货执行完成")
}

func (s *AppServer) shipOrderHandler(c *gin.Context) {
	var req ShipOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.Username == "" {
		respondError(c, http.StatusBadRequest, "MISSING_USERNAME", "缺少用户名参数", "username is required")
		return
	}

	result, err := s.xianyuService.ShipOrder(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SHIP_ORDER_FAILED", "发货操作失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "发货操作执行完成")
}

func healthHandler(c *gin.Context) {
	respondSuccess(c, map[string]any{
		"status":    "healthy",
		"service":   "xianyu-mcp",
		"timestamp": time.Now().Format(time.RFC3339),
	}, "服务正常")
}
