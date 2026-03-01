package main

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
