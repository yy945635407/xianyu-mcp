package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
)

func boolPtr(b bool) *bool { return &b }

type SearchItemsArgs struct {
	Keyword string `json:"keyword" jsonschema:"搜索关键词"`
	Limit   int    `json:"limit,omitempty" jsonschema:"返回条数限制，默认20，最大100"`
}

func InitMCPServer(appServer *AppServer) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "xianyu-mcp",
			Version: "0.1.0",
		},
		nil,
	)

	registerTools(server, appServer)
	logrus.Info("MCP Server initialized with official SDK")
	return server
}

func withPanicRecovery[T any](
	toolName string,
	handler func(context.Context, *mcp.CallToolRequest, T) (*mcp.CallToolResult, any, error),
) func(context.Context, *mcp.CallToolRequest, T) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args T) (result *mcp.CallToolResult, resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithFields(logrus.Fields{"tool": toolName, "panic": r}).Error("Tool handler panicked")
				logrus.Errorf("Stack trace:\n%s", debug.Stack())
				result = &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("工具 %s 执行时发生内部错误: %v", toolName, r)}},
					IsError: true,
				}
				resp = nil
				err = nil
			}
		}()
		return handler(ctx, req, args)
	}
}

func registerTools(server *mcp.Server, appServer *AppServer) {
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "check_login_status",
			Description: "检查闲鱼登录状态",
			Annotations: &mcp.ToolAnnotations{Title: "Check Login Status", ReadOnlyHint: true},
		},
		withPanicRecovery("check_login_status", func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			result := appServer.handleCheckLoginStatus(ctx)
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_login_qrcode",
			Description: "获取闲鱼登录二维码（返回 Base64 图片和超时时间）",
			Annotations: &mcp.ToolAnnotations{Title: "Get Login QR Code", ReadOnlyHint: true},
		},
		withPanicRecovery("get_login_qrcode", func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			result := appServer.handleGetLoginQrcode(ctx)
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "delete_cookies",
			Description: "删除 cookies 文件，重置登录状态",
			Annotations: &mcp.ToolAnnotations{Title: "Delete Cookies", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("delete_cookies", func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			result := appServer.handleDeleteCookies(ctx)
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "search_items",
			Description: "搜索闲鱼商品，返回商品摘要列表",
			Annotations: &mcp.ToolAnnotations{Title: "Search Items", ReadOnlyHint: true},
		},
		withPanicRecovery("search_items", func(ctx context.Context, req *mcp.CallToolRequest, args SearchItemsArgs) (*mcp.CallToolResult, any, error) {
			if args.Keyword == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 keyword 参数"}},
				}, nil, nil
			}

			result := appServer.handleSearchItems(ctx, args.Keyword, args.Limit)
			return convertToMCPResult(result), nil, nil
		}),
	)

	logrus.Info("Registered 4 MCP tools")
}

func convertToMCPResult(result *MCPToolResult) *mcp.CallToolResult {
	var contents []mcp.Content
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			contents = append(contents, &mcp.TextContent{Text: c.Text})
		case "image":
			imageData, err := base64.StdEncoding.DecodeString(c.Data)
			if err != nil {
				contents = append(contents, &mcp.TextContent{Text: "图片数据解码失败: " + err.Error()})
				continue
			}
			contents = append(contents, &mcp.ImageContent{Data: imageData, MIMEType: c.MimeType})
		}
	}

	return &mcp.CallToolResult{Content: contents, IsError: result.IsError}
}
