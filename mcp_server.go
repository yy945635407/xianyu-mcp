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

type ListConversationsArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"返回会话条数限制，默认20"`
}

type GetMessagesArgs struct {
	Username string `json:"username" jsonschema:"会话用户名"`
	Limit    int    `json:"limit,omitempty" jsonschema:"返回消息条数限制，默认50"`
}

type SendMessageArgs struct {
	Username string `json:"username" jsonschema:"会话用户名"`
	Message  string `json:"message" jsonschema:"要发送的消息内容"`
	Limit    int    `json:"limit,omitempty" jsonschema:"发送后返回最近消息条数限制，默认30"`
}

type PublishItemArgs struct {
	Images            []string `json:"images" jsonschema:"本地图片绝对路径列表（至少1张）"`
	Description       string   `json:"description" jsonschema:"宝贝描述（网页必填）"`
	Price             string   `json:"price" jsonschema:"售价（网页必填）"`
	OriginalPrice     string   `json:"original_price,omitempty" jsonschema:"原价（可选）"`
	ShippingType      string   `json:"shipping_type,omitempty" jsonschema:"发货方式：包邮|按距离计费|一口价|无需邮寄"`
	ShippingFee       string   `json:"shipping_fee,omitempty" jsonschema:"邮费（按距离计费/一口价时建议填写）"`
	SupportSelfPickup bool     `json:"support_self_pickup,omitempty" jsonschema:"是否支持自提"`
	LocationKeyword   string   `json:"location_keyword,omitempty" jsonschema:"地址关键字，匹配“宝贝所在地”候选地址"`
	SpecTypes         []string `json:"spec_types,omitempty" jsonschema:"商品规格类型列表（可选，最多2个）"`
	Submit            bool     `json:"submit,omitempty" jsonschema:"是否实际点击发布。false仅填表校验，true执行发布"`
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

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "list_conversations",
			Description: "读取闲鱼消息会话列表，返回用户名、最新消息、交易状态",
			Annotations: &mcp.ToolAnnotations{Title: "List Conversations", ReadOnlyHint: true},
		},
		withPanicRecovery("list_conversations", func(ctx context.Context, req *mcp.CallToolRequest, args ListConversationsArgs) (*mcp.CallToolResult, any, error) {
			result := appServer.handleListConversations(ctx, args.Limit)
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_messages",
			Description: "按用户名查询会话消息，返回消息列表、商品上下文和订单状态（未下单/已拍下/我已发货/已收货）",
			Annotations: &mcp.ToolAnnotations{Title: "Get Messages", ReadOnlyHint: true},
		},
		withPanicRecovery("get_messages", func(ctx context.Context, req *mcp.CallToolRequest, args GetMessagesArgs) (*mcp.CallToolResult, any, error) {
			if args.Username == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 username 参数"}},
				}, nil, nil
			}
			result := appServer.handleGetMessages(ctx, args.Username, args.Limit)
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "send_message",
			Description: "按用户名发送消息，并返回发送后会话摘要",
			Annotations: &mcp.ToolAnnotations{Title: "Send Message", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("send_message", func(ctx context.Context, req *mcp.CallToolRequest, args SendMessageArgs) (*mcp.CallToolResult, any, error) {
			if args.Username == "" || args.Message == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 username 或 message 参数"}},
				}, nil, nil
			}
			result := appServer.handleSendMessage(ctx, args.Username, args.Message, args.Limit)
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "publish_item",
			Description: "发布闲置商品，参数覆盖网页可填写字段。默认建议 submit=false 先做填表校验。",
			Annotations: &mcp.ToolAnnotations{Title: "Publish Item", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("publish_item", func(ctx context.Context, req *mcp.CallToolRequest, args PublishItemArgs) (*mcp.CallToolResult, any, error) {
			if len(args.Images) == 0 || args.Description == "" || args.Price == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少必填参数：images/description/price"}},
				}, nil, nil
			}

			result := appServer.handlePublishItem(ctx, PublishItemRequest{
				Images:            args.Images,
				Description:       args.Description,
				Price:             args.Price,
				OriginalPrice:     args.OriginalPrice,
				ShippingType:      args.ShippingType,
				ShippingFee:       args.ShippingFee,
				SupportSelfPickup: args.SupportSelfPickup,
				LocationKeyword:   args.LocationKeyword,
				SpecTypes:         args.SpecTypes,
				Submit:            args.Submit,
			})
			return convertToMCPResult(result), nil, nil
		}),
	)

	logrus.Info("Registered 8 MCP tools")
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
