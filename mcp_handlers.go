package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ylyt_bot/xianyu-mcp/cookies"
)

func (s *AppServer) handleCheckLoginStatus(ctx context.Context) *MCPToolResult {
	status, err := s.xianyuService.CheckLoginStatus(ctx)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "检查登录状态失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	if status.IsLoggedIn {
		return &MCPToolResult{Content: []MCPContent{{
			Type: "text",
			Text: fmt.Sprintf("✅ 已登录\n用户名: %s", status.Username),
		}}}
	}

	return &MCPToolResult{Content: []MCPContent{{
		Type: "text",
		Text: "❌ 未登录\n请调用 get_login_qrcode 获取二维码后扫码登录。",
	}}}
}

func (s *AppServer) handleGetLoginQrcode(ctx context.Context) *MCPToolResult {
	result, err := s.xianyuService.GetLoginQrcode(ctx)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "获取登录扫码图片失败: " + err.Error()}},
			IsError: true,
		}
	}

	if result.IsLoggedIn {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "你当前已处于登录状态"}}}
	}

	now := time.Now()
	deadline := formatLoginDeadline(now, result.Timeout)
	contents := []MCPContent{
		{Type: "text", Text: loginInstructionText(deadline)},
		{
			Type:     "image",
			MimeType: "image/png",
			Data:     normalizeQRCodeData(result.Img),
		},
	}
	return &MCPToolResult{Content: contents}
}

func (s *AppServer) handleDeleteCookies(ctx context.Context) *MCPToolResult {
	err := s.xianyuService.DeleteCookies(ctx)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "删除 cookies 失败: " + err.Error()}},
			IsError: true,
		}
	}

	cookiePath := cookies.GetCookiesFilePath()
	return &MCPToolResult{Content: []MCPContent{{
		Type: "text",
		Text: fmt.Sprintf("Cookies 已成功删除，登录状态已重置。\n\n删除路径: %s", cookiePath),
	}}}
}

func (s *AppServer) handleSearchItems(ctx context.Context, keyword string, limit int) *MCPToolResult {
	result, err := s.xianyuService.SearchItems(ctx, keyword, limit)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "搜索商品失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "搜索成功但结果序列化失败: " + err.Error()}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{Type: "text", Text: string(data)}},
	}
}

func (s *AppServer) handleListConversations(ctx context.Context, limit int) *MCPToolResult {
	result, err := s.xianyuService.ListConversations(ctx, limit)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "读取消息列表失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "读取成功但序列化失败: " + err.Error()}},
			IsError: true,
		}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func (s *AppServer) handleGetMessages(ctx context.Context, username string, limit int) *MCPToolResult {
	result, err := s.xianyuService.GetConversationMessages(ctx, username, limit)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "查询消息失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "查询成功但序列化失败: " + err.Error()}},
			IsError: true,
		}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func (s *AppServer) handleSendMessage(ctx context.Context, username, message string, limit int) *MCPToolResult {
	result, err := s.xianyuService.SendMessage(ctx, username, message, limit)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "发送消息失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "发送成功但序列化失败: " + err.Error()}},
			IsError: true,
		}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func (s *AppServer) handlePublishItem(ctx context.Context, req PublishItemRequest) *MCPToolResult {
	result, err := s.xianyuService.PublishItem(ctx, &req)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "发布闲置失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "发布流程执行成功但序列化失败: " + err.Error()}},
			IsError: true,
		}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func (s *AppServer) handleListOrders(ctx context.Context, tab string, limit int) *MCPToolResult {
	result, err := s.xianyuService.ListOrders(ctx, tab, limit)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "查询订单失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "查询成功但序列化失败: " + err.Error()}},
			IsError: true,
		}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func (s *AppServer) handleRemindShip(ctx context.Context, req RemindShipRequest) *MCPToolResult {
	result, err := s.xianyuService.RemindShip(ctx, &req)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "提醒发货失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "提醒成功但序列化失败: " + err.Error()}},
			IsError: true,
		}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func (s *AppServer) handleShipOrder(ctx context.Context, req ShipOrderRequest) *MCPToolResult {
	result, err := s.xianyuService.ShipOrder(ctx, &req)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "发货失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "发货执行完成但序列化失败: " + err.Error()}},
			IsError: true,
		}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func (s *AppServer) handleListCollections(ctx context.Context, group string, limit int) *MCPToolResult {
	result, err := s.xianyuService.ListCollections(ctx, group, limit)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "读取收藏夹失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "读取成功但序列化失败: " + err.Error()}},
			IsError: true,
		}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func (s *AppServer) handleCancelFavorite(ctx context.Context, req CancelFavoriteRequest) *MCPToolResult {
	result, err := s.xianyuService.CancelFavorite(ctx, &req)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "取消收藏失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "取消收藏执行完成但序列化失败: " + err.Error()}},
			IsError: true,
		}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func (s *AppServer) handleManageCollectionGroup(ctx context.Context, req ManageCollectionGroupRequest) *MCPToolResult {
	result, err := s.xianyuService.ManageCollectionGroup(ctx, &req)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "分组管理失败: " + err.Error()}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "分组管理执行完成但序列化失败: " + err.Error()}},
			IsError: true,
		}
	}
	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(data)}}}
}

func normalizeQRCodeData(img string) string {
	trimmed := strings.TrimSpace(img)
	trimmed = strings.TrimPrefix(trimmed, "data:image/png;base64,")
	trimmed = strings.TrimPrefix(trimmed, "data:image/jpeg;base64,")
	return trimmed
}
