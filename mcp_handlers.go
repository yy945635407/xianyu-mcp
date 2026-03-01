package main

import (
	"context"
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

func normalizeQRCodeData(img string) string {
	trimmed := strings.TrimSpace(img)
	trimmed = strings.TrimPrefix(trimmed, "data:image/png;base64,")
	trimmed = strings.TrimPrefix(trimmed, "data:image/jpeg;base64,")
	return trimmed
}
