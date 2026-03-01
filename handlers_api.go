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

func (s *AppServer) listCollectionsHandler(c *gin.Context) {
	group := c.Query("group")
	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_LIMIT", "limit 必须是整数", err.Error())
			return
		}
		limit = parsed
	}

	result, err := s.xianyuService.ListCollections(c.Request.Context(), group, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIST_COLLECTIONS_FAILED", "读取收藏夹失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "读取收藏夹成功")
}

func (s *AppServer) cancelFavoriteHandler(c *gin.Context) {
	var req CancelFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.Keyword == "" && req.ItemRef == "" {
		respondError(c, http.StatusBadRequest, "MISSING_PARAMS", "缺少参数", "keyword or item_ref is required")
		return
	}

	result, err := s.xianyuService.CancelFavorite(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "CANCEL_FAVORITE_FAILED", "取消收藏失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "取消收藏执行完成")
}

func (s *AppServer) manageCollectionGroupHandler(c *gin.Context) {
	var req ManageCollectionGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}

	result, err := s.xianyuService.ManageCollectionGroup(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "MANAGE_COLLECTION_GROUP_FAILED", "分组管理失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "分组管理执行完成")
}

func (s *AppServer) listMyItemsHandler(c *gin.Context) {
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

	result, err := s.xianyuService.ListMyItems(c.Request.Context(), tab, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIST_MY_ITEMS_FAILED", "读取我的宝贝失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "读取我的宝贝成功")
}

func (s *AppServer) editMyItemHandler(c *gin.Context) {
	var req EditMyItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.Keyword == "" && req.ItemRef == "" {
		respondError(c, http.StatusBadRequest, "MISSING_PARAMS", "缺少参数", "keyword or item_ref is required")
		return
	}

	result, err := s.xianyuService.EditMyItem(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "EDIT_MY_ITEM_FAILED", "编辑宝贝失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "编辑宝贝执行完成")
}

func (s *AppServer) shelfMyItemHandler(c *gin.Context) {
	var req ShelfMyItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.Keyword == "" && req.ItemRef == "" {
		respondError(c, http.StatusBadRequest, "MISSING_PARAMS", "缺少参数", "keyword or item_ref is required")
		return
	}

	result, err := s.xianyuService.ShelfMyItem(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SHELF_MY_ITEM_FAILED", "上下架操作失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "上下架操作执行完成")
}

func (s *AppServer) deleteMyItemHandler(c *gin.Context) {
	var req DeleteMyItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.Keyword == "" && req.ItemRef == "" {
		respondError(c, http.StatusBadRequest, "MISSING_PARAMS", "缺少参数", "keyword or item_ref is required")
		return
	}

	result, err := s.xianyuService.DeleteMyItem(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_MY_ITEM_FAILED", "删除宝贝失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "删除宝贝执行完成")
}

func (s *AppServer) getItemDetailHandler(c *gin.Context) {
	itemRef := c.Query("item_ref")
	if itemRef == "" {
		respondError(c, http.StatusBadRequest, "MISSING_ITEM_REF", "缺少商品参数", "item_ref is required")
		return
	}

	result, err := s.xianyuService.GetItemDetail(c.Request.Context(), itemRef)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_ITEM_DETAIL_FAILED", "读取商品详情失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "读取商品详情成功")
}

func (s *AppServer) favoriteItemHandler(c *gin.Context) {
	var req FavoriteItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.ItemRef == "" {
		respondError(c, http.StatusBadRequest, "MISSING_ITEM_REF", "缺少商品参数", "item_ref is required")
		return
	}

	result, err := s.xianyuService.FavoriteItem(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FAVORITE_ITEM_FAILED", "收藏操作失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "收藏操作执行完成")
}

func (s *AppServer) chatItemHandler(c *gin.Context) {
	var req ChatItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.ItemRef == "" {
		respondError(c, http.StatusBadRequest, "MISSING_ITEM_REF", "缺少商品参数", "item_ref is required")
		return
	}

	result, err := s.xianyuService.ChatItem(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "CHAT_ITEM_FAILED", "聊一聊操作失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "聊一聊操作执行完成")
}

func (s *AppServer) buyItemHandler(c *gin.Context) {
	var req BuyItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.ItemRef == "" {
		respondError(c, http.StatusBadRequest, "MISSING_ITEM_REF", "缺少商品参数", "item_ref is required")
		return
	}

	result, err := s.xianyuService.BuyItem(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "BUY_ITEM_FAILED", "立即购买操作失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "立即购买操作执行完成")
}

func (s *AppServer) getAccountSecurityHandler(c *gin.Context) {
	result, err := s.xianyuService.GetAccountSecurity(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_ACCOUNT_SECURITY_FAILED", "读取账号与安全信息失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "读取账号与安全信息成功")
}

func (s *AppServer) getCommunityFeedHandler(c *gin.Context) {
	keyword := c.Query("keyword")
	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_LIMIT", "limit 必须是整数", err.Error())
			return
		}
		limit = parsed
	}

	result, err := s.xianyuService.GetCommunityFeed(c.Request.Context(), keyword, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_COMMUNITY_FEED_FAILED", "读取社区内容失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "读取社区内容成功")
}

func (s *AppServer) interactCommunityHandler(c *gin.Context) {
	var req InteractCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.Keyword == "" {
		respondError(c, http.StatusBadRequest, "MISSING_KEYWORD", "缺少关键词参数", "keyword is required")
		return
	}

	result, err := s.xianyuService.InteractCommunity(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERACT_COMMUNITY_FAILED", "社区互动失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "社区互动执行完成")
}

func (s *AppServer) getCustomerServiceHandler(c *gin.Context) {
	afterSaleLimit := 20
	if limitStr := c.Query("after_sale_limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_LIMIT", "after_sale_limit 必须是整数", err.Error())
			return
		}
		afterSaleLimit = parsed
	}

	result, err := s.xianyuService.GetCustomerService(c.Request.Context(), afterSaleLimit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_CUSTOMER_SERVICE_FAILED", "读取客服/售后信息失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "读取客服/售后信息成功")
}

func (s *AppServer) openCustomerServiceHandler(c *gin.Context) {
	var req OpenCustomerServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}

	result, err := s.xianyuService.OpenCustomerService(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "OPEN_CUSTOMER_SERVICE_FAILED", "打开客服入口失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "打开客服入口执行完成")
}

func (s *AppServer) shipWithLogisticsHandler(c *gin.Context) {
	var req ShipWithLogisticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}
	if req.Username == "" || req.TrackingNo == "" {
		respondError(c, http.StatusBadRequest, "MISSING_PARAMS", "缺少参数", "username and tracking_no are required")
		return
	}

	result, err := s.xianyuService.ShipWithLogistics(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SHIP_WITH_LOGISTICS_FAILED", "物流发货失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "物流发货执行完成")
}

func (s *AppServer) confirmReceiptHandler(c *gin.Context) {
	var req ConfirmReceiptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}

	result, err := s.xianyuService.ConfirmReceipt(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "CONFIRM_RECEIPT_FAILED", "确认收货失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "确认收货执行完成")
}

func (s *AppServer) reviewOrderHandler(c *gin.Context) {
	var req ReviewOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}

	result, err := s.xianyuService.ReviewOrder(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "REVIEW_ORDER_FAILED", "评价订单失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "评价订单执行完成")
}

func (s *AppServer) refundActionHandler(c *gin.Context) {
	var req RefundActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数错误", err.Error())
		return
	}

	result, err := s.xianyuService.HandleRefund(c.Request.Context(), &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "REFUND_ACTION_FAILED", "退款处理失败", err.Error())
		return
	}

	c.Set("account", "xianyu-mcp")
	respondSuccess(c, result, "退款处理执行完成")
}

func healthHandler(c *gin.Context) {
	respondSuccess(c, map[string]any{
		"status":    "healthy",
		"service":   "xianyu-mcp",
		"timestamp": time.Now().Format(time.RFC3339),
	}, "服务正常")
}
