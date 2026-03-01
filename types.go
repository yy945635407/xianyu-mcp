package main

import "github.com/ylyt_bot/xianyu-mcp/xianyu"

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details any    `json:"details,omitempty"`
}

type SuccessResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Message string `json:"message,omitempty"`
}

type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type MCPContent struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type LoginStatusResponse struct {
	IsLoggedIn bool   `json:"is_logged_in"`
	Username   string `json:"username,omitempty"`
}

type LoginQrcodeResponse struct {
	Timeout    string `json:"timeout"`
	IsLoggedIn bool   `json:"is_logged_in"`
	Img        string `json:"img,omitempty"`
}

type SearchItemsRequest struct {
	Keyword string `json:"keyword" binding:"required"`
	Limit   int    `json:"limit,omitempty"`
}

type SearchItemResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Price     string `json:"price,omitempty"`
	WantCount int    `json:"want_count,omitempty"`
	URL       string `json:"url"`
	Seller    string `json:"seller,omitempty"`
}

type SearchItemsResponse struct {
	Keyword string               `json:"keyword"`
	Count   int                  `json:"count"`
	Items   []SearchItemResponse `json:"items"`
}

type ListConversationsRequest struct {
	Limit int `json:"limit,omitempty"`
}

type GetMessagesRequest struct {
	Username string `json:"username" binding:"required"`
	Limit    int    `json:"limit,omitempty"`
}

type SendMessageRequest struct {
	Username string `json:"username" binding:"required"`
	Message  string `json:"message" binding:"required"`
	Limit    int    `json:"limit,omitempty"`
}

type ListConversationsResponse struct {
	Count         int                          `json:"count"`
	Conversations []xianyu.ConversationSummary `json:"conversations"`
}

type GetMessagesResponse struct {
	Conversation xianyu.ConversationDetail `json:"conversation"`
}

type SendMessageResponse struct {
	Username     string                    `json:"username"`
	Message      string                    `json:"message"`
	Sent         bool                      `json:"sent"`
	Conversation xianyu.ConversationDetail `json:"conversation"`
}

type PublishItemRequest struct {
	Images            []string `json:"images" binding:"required,min=1"`
	Description       string   `json:"description" binding:"required"`
	Price             string   `json:"price" binding:"required"`
	OriginalPrice     string   `json:"original_price,omitempty"`
	ShippingType      string   `json:"shipping_type,omitempty"` // 包邮|按距离计费|一口价|无需邮寄
	ShippingFee       string   `json:"shipping_fee,omitempty"`
	SupportSelfPickup bool     `json:"support_self_pickup,omitempty"`
	LocationKeyword   string   `json:"location_keyword,omitempty"`
	SpecTypes         []string `json:"spec_types,omitempty"`
	Submit            bool     `json:"submit,omitempty"`
}

type PublishItemResponse struct {
	Result xianyu.PublishItemResult `json:"result"`
}
