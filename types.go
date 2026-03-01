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
	Username    string `json:"username" binding:"required"`
	Message     string `json:"message" binding:"required"`
	Limit       int    `json:"limit,omitempty"`
	ClientMsgID string `json:"client_msg_id,omitempty"`
	MaxRetries  int    `json:"max_retries,omitempty"`
	Force       bool   `json:"force,omitempty"`
}

type PullIMEventsRequest struct {
	SinceID   int64 `json:"since_id,omitempty"`
	Limit     int   `json:"limit,omitempty"`
	ScanLimit int   `json:"scan_limit,omitempty"`
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
	ClientMsgID  string                    `json:"client_msg_id,omitempty"`
	Attempts     int                       `json:"attempts,omitempty"`
	Deduplicated bool                      `json:"deduplicated,omitempty"`
	Blocked      bool                      `json:"blocked,omitempty"`
	BlockReason  string                    `json:"block_reason,omitempty"`
	Conversation xianyu.ConversationDetail `json:"conversation"`
}

type PullIMEventsResponse struct {
	SinceID    int64     `json:"since_id"`
	NextCursor int64     `json:"next_cursor"`
	Generated  int       `json:"generated"`
	Count      int       `json:"count"`
	Events     []IMEvent `json:"events"`
}

type SetIMSessionStateRequest struct {
	Username      string `json:"username" binding:"required"`
	Mode          string `json:"mode,omitempty"` // bot|human
	HandoffReason string `json:"handoff_reason,omitempty"`
	LockOwner     string `json:"lock_owner,omitempty"`
	LockSeconds   int64  `json:"lock_seconds,omitempty"`
	ClearLock     bool   `json:"clear_lock,omitempty"`
}

type MarkIMSessionReadRequest struct {
	Username string `json:"username" binding:"required"`
	Limit    int    `json:"limit,omitempty"`
}

type IMSessionStateResponse struct {
	State IMSessionState `json:"state"`
}

type IMSessionStatesResponse struct {
	Count  int              `json:"count"`
	States []IMSessionState `json:"states"`
}

type UpsertIMKnowledgeRequest struct {
	ID            string   `json:"id,omitempty"`
	Title         string   `json:"title,omitempty"`
	Keywords      []string `json:"keywords" binding:"required,min=1"`
	Answer        string   `json:"answer" binding:"required"`
	ItemRef       string   `json:"item_ref,omitempty"`
	OrderStatuses []string `json:"order_statuses,omitempty"` // 未下单|已拍下|我已发货|已收货
	Tags          []string `json:"tags,omitempty"`
	Enabled       *bool    `json:"enabled,omitempty"`
	Priority      int      `json:"priority,omitempty"`
}

type ListIMKnowledgeRequest struct {
	ItemRef     string `json:"item_ref,omitempty"`
	OrderStatus string `json:"order_status,omitempty"`
	Query       string `json:"query,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type DeleteIMKnowledgeRequest struct {
	ID string `json:"id" binding:"required"`
}

type MatchIMKnowledgeRequest struct {
	Message     string `json:"message" binding:"required"`
	Username    string `json:"username,omitempty"`
	ItemRef     string `json:"item_ref,omitempty"`
	OrderStatus string `json:"order_status,omitempty"`
	TopK        int    `json:"top_k,omitempty"`
	AutoContext bool   `json:"auto_context,omitempty"`
}

type IMKnowledgeEntryResponse struct {
	Entry IMKnowledgeEntry `json:"entry"`
}

type IMKnowledgeListResponse struct {
	Count   int                `json:"count"`
	Entries []IMKnowledgeEntry `json:"entries"`
}

type DeleteIMKnowledgeResponse struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type IMKnowledgeMatch struct {
	ID              string   `json:"id"`
	Title           string   `json:"title,omitempty"`
	Answer          string   `json:"answer"`
	ItemRef         string   `json:"item_ref,omitempty"`
	OrderStatuses   []string `json:"order_statuses,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Score           int      `json:"score"`
	MatchedKeywords []string `json:"matched_keywords,omitempty"`
}

type MatchIMKnowledgeResponse struct {
	Message     string             `json:"message"`
	Username    string             `json:"username,omitempty"`
	ItemRef     string             `json:"item_ref,omitempty"`
	OrderStatus string             `json:"order_status,omitempty"`
	Count       int                `json:"count"`
	BestAnswer  string             `json:"best_answer,omitempty"`
	Matches     []IMKnowledgeMatch `json:"matches"`
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

type ListOrdersRequest struct {
	Tab   string `json:"tab,omitempty"` // 全部|待付款|待发货|待收货|待评价|退款中
	Limit int    `json:"limit,omitempty"`
}

type ListOrdersResponse struct {
	Count  int                   `json:"count"`
	Orders []xianyu.OrderSummary `json:"orders"`
}

type RemindShipRequest struct {
	OrderKeyword string `json:"order_keyword,omitempty"`
	SellerName   string `json:"seller_name,omitempty"`
}

type ShipOrderRequest struct {
	Username string `json:"username" binding:"required"`
}

type OrderActionResponse struct {
	Result xianyu.OrderActionResult `json:"result"`
}

type ListCollectionsRequest struct {
	Group string `json:"group,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type CancelFavoriteRequest struct {
	Keyword string `json:"keyword,omitempty"`
	ItemRef string `json:"item_ref,omitempty"`
}

type ManageCollectionGroupRequest struct {
	Operation   string `json:"operation" binding:"required"` // create|rename|delete|move
	GroupName   string `json:"group_name,omitempty"`
	NewName     string `json:"new_name,omitempty"`
	ItemKeyword string `json:"item_keyword,omitempty"`
}

type ListCollectionsResponse struct {
	Result xianyu.CollectionListResult `json:"result"`
}

type CollectionActionResponse struct {
	Result xianyu.CollectionActionResult `json:"result"`
}

type ListMyItemsRequest struct {
	Tab   string `json:"tab,omitempty"` // 在售|已售出|下架
	Limit int    `json:"limit,omitempty"`
}

type EditMyItemRequest struct {
	Keyword     string `json:"keyword,omitempty"`
	ItemRef     string `json:"item_ref,omitempty"` // 商品链接或商品ID
	Tab         string `json:"tab,omitempty"`      // 在售|已售出|下架
	Price       string `json:"price,omitempty"`
	Description string `json:"description,omitempty"`
	Submit      bool   `json:"submit,omitempty"` // 是否点击保存/发布
}

type ShelfMyItemRequest struct {
	Keyword string `json:"keyword,omitempty"`
	ItemRef string `json:"item_ref,omitempty"`
	Tab     string `json:"tab,omitempty"`
	Action  string `json:"action,omitempty"` // up|down|auto
}

type DeleteMyItemRequest struct {
	Keyword string `json:"keyword,omitempty"`
	ItemRef string `json:"item_ref,omitempty"`
	Tab     string `json:"tab,omitempty"`
}

type ListMyItemsResponse struct {
	Result xianyu.MyItemListResult `json:"result"`
}

type MyItemActionResponse struct {
	Result xianyu.MyItemActionResult `json:"result"`
}

type GetItemDetailRequest struct {
	ItemRef string `json:"item_ref" binding:"required"` // 商品链接或商品ID
}

type FavoriteItemRequest struct {
	ItemRef string `json:"item_ref" binding:"required"`
}

type ChatItemRequest struct {
	ItemRef string `json:"item_ref" binding:"required"`
	Message string `json:"message,omitempty"`
}

type BuyItemRequest struct {
	ItemRef string `json:"item_ref" binding:"required"`
}

type GetItemDetailResponse struct {
	Result xianyu.ItemDetail `json:"result"`
}

type ItemOperateResponse struct {
	Result xianyu.ItemActionResult `json:"result"`
}

type AccountSecurityResponse struct {
	Result xianyu.AccountSecurityInfo `json:"result"`
}

type CommunityFeedRequest struct {
	Keyword string `json:"keyword,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

type InteractCommunityRequest struct {
	Keyword string `json:"keyword" binding:"required"`
	Action  string `json:"action,omitempty"` // open_item|open_category
}

type OpenCustomerServiceRequest struct {
	Name string `json:"name,omitempty"` // 客服|反馈
}

type CustomerServiceRequest struct {
	AfterSaleLimit int `json:"after_sale_limit,omitempty"`
}

type CommunityFeedResponse struct {
	Result xianyu.CommunityFeedResult `json:"result"`
}

type CommunityActionResponse struct {
	Result xianyu.CommunityActionResult `json:"result"`
}

type CustomerServiceResponse struct {
	Result xianyu.CustomerServiceResult `json:"result"`
}

type ShipWithLogisticsRequest struct {
	Username   string `json:"username" binding:"required"`
	Company    string `json:"company,omitempty"`
	TrackingNo string `json:"tracking_no" binding:"required"`
}

type ConfirmReceiptRequest struct {
	OrderKeyword string `json:"order_keyword,omitempty"`
	SellerName   string `json:"seller_name,omitempty"`
}

type ReviewOrderRequest struct {
	OrderKeyword string `json:"order_keyword,omitempty"`
	SellerName   string `json:"seller_name,omitempty"`
	Score        int    `json:"score,omitempty"` // 1-5
	Content      string `json:"content,omitempty"`
}

type RefundActionRequest struct {
	OrderKeyword string `json:"order_keyword,omitempty"`
	SellerName   string `json:"seller_name,omitempty"`
	Action       string `json:"action,omitempty"` // detail|contact|complaint|money|snapshot|delete
	Reason       string `json:"reason,omitempty"`
}
