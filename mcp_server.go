package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"runtime/debug"
	"strings"

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

type ListOrdersArgs struct {
	Tab   string `json:"tab,omitempty" jsonschema:"订单页签：全部|待付款|待发货|待收货|待评价|退款中"`
	Limit int    `json:"limit,omitempty" jsonschema:"返回条数限制，默认20"`
}

type RemindShipArgs struct {
	OrderKeyword string `json:"order_keyword,omitempty" jsonschema:"订单关键字（商品标题片段）"`
	SellerName   string `json:"seller_name,omitempty" jsonschema:"卖家名称（可选）"`
}

type ShipOrderArgs struct {
	Username string `json:"username" jsonschema:"买家用户名（IM 会话用户名）"`
}

type ListCollectionsArgs struct {
	Group string `json:"group,omitempty" jsonschema:"收藏分组名称（默认全部）"`
	Limit int    `json:"limit,omitempty" jsonschema:"返回条数限制，默认20"`
}

type CancelFavoriteArgs struct {
	Keyword string `json:"keyword,omitempty" jsonschema:"商品标题关键字（和 item_ref 至少一个）"`
	ItemRef string `json:"item_ref,omitempty" jsonschema:"商品链接或商品ID（和 keyword 至少一个）"`
}

type ManageCollectionGroupArgs struct {
	Operation   string `json:"operation" jsonschema:"分组操作：create|rename|delete|move"`
	GroupName   string `json:"group_name,omitempty" jsonschema:"分组名称（rename/delete/move 需要）"`
	NewName     string `json:"new_name,omitempty" jsonschema:"新分组名（create/rename 可用）"`
	ItemKeyword string `json:"item_keyword,omitempty" jsonschema:"商品关键字（move 需要）"`
}

type ListMyItemsArgs struct {
	Tab   string `json:"tab,omitempty" jsonschema:"页签：在售|已售出|下架"`
	Limit int    `json:"limit,omitempty" jsonschema:"返回条数限制，默认20"`
}

type EditMyItemArgs struct {
	Keyword     string `json:"keyword,omitempty" jsonschema:"商品标题关键字（与 item_ref 至少一个）"`
	ItemRef     string `json:"item_ref,omitempty" jsonschema:"商品链接或商品ID（与 keyword 至少一个）"`
	Tab         string `json:"tab,omitempty" jsonschema:"页签：在售|已售出|下架"`
	Price       string `json:"price,omitempty" jsonschema:"新价格（可选）"`
	Description string `json:"description,omitempty" jsonschema:"新描述（可选）"`
	Submit      bool   `json:"submit,omitempty" jsonschema:"是否尝试点击保存/发布"`
}

type ShelfMyItemArgs struct {
	Keyword string `json:"keyword,omitempty" jsonschema:"商品标题关键字（与 item_ref 至少一个）"`
	ItemRef string `json:"item_ref,omitempty" jsonschema:"商品链接或商品ID（与 keyword 至少一个）"`
	Tab     string `json:"tab,omitempty" jsonschema:"页签：在售|已售出|下架"`
	Action  string `json:"action,omitempty" jsonschema:"操作：up|down|auto"`
}

type DeleteMyItemArgs struct {
	Keyword string `json:"keyword,omitempty" jsonschema:"商品标题关键字（与 item_ref 至少一个）"`
	ItemRef string `json:"item_ref,omitempty" jsonschema:"商品链接或商品ID（与 keyword 至少一个）"`
	Tab     string `json:"tab,omitempty" jsonschema:"页签：在售|已售出|下架"`
}

type GetItemDetailArgs struct {
	ItemRef string `json:"item_ref" jsonschema:"商品链接或商品ID"`
}

type FavoriteItemArgs struct {
	ItemRef string `json:"item_ref" jsonschema:"商品链接或商品ID"`
}

type ChatItemArgs struct {
	ItemRef string `json:"item_ref" jsonschema:"商品链接或商品ID"`
	Message string `json:"message,omitempty" jsonschema:"可选，打开会话后自动发送的首条消息"`
}

type BuyItemArgs struct {
	ItemRef string `json:"item_ref" jsonschema:"商品链接或商品ID"`
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

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "list_orders",
			Description: "查询买到订单列表，可按页签筛选",
			Annotations: &mcp.ToolAnnotations{Title: "List Orders", ReadOnlyHint: true},
		},
		withPanicRecovery("list_orders", func(ctx context.Context, req *mcp.CallToolRequest, args ListOrdersArgs) (*mcp.CallToolResult, any, error) {
			result := appServer.handleListOrders(ctx, args.Tab, args.Limit)
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "remind_ship",
			Description: "在买到订单中触发“提醒发货”",
			Annotations: &mcp.ToolAnnotations{Title: "Remind Ship", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("remind_ship", func(ctx context.Context, req *mcp.CallToolRequest, args RemindShipArgs) (*mcp.CallToolResult, any, error) {
			result := appServer.handleRemindShip(ctx, RemindShipRequest{
				OrderKeyword: args.OrderKeyword,
				SellerName:   args.SellerName,
			})
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "ship_order",
			Description: "在 IM 会话里触发“去发货”，若网页端受限会返回 requires_app=true",
			Annotations: &mcp.ToolAnnotations{Title: "Ship Order", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("ship_order", func(ctx context.Context, req *mcp.CallToolRequest, args ShipOrderArgs) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(args.Username) == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 username 参数"}},
				}, nil, nil
			}
			result := appServer.handleShipOrder(ctx, ShipOrderRequest{
				Username: args.Username,
			})
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "list_collections",
			Description: "读取收藏夹商品列表与分组信息",
			Annotations: &mcp.ToolAnnotations{Title: "List Collections", ReadOnlyHint: true},
		},
		withPanicRecovery("list_collections", func(ctx context.Context, req *mcp.CallToolRequest, args ListCollectionsArgs) (*mcp.CallToolResult, any, error) {
			result := appServer.handleListCollections(ctx, args.Group, args.Limit)
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "cancel_favorite",
			Description: "取消收藏商品（通过关键词或商品链接/ID匹配）",
			Annotations: &mcp.ToolAnnotations{Title: "Cancel Favorite", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("cancel_favorite", func(ctx context.Context, req *mcp.CallToolRequest, args CancelFavoriteArgs) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(args.Keyword) == "" && strings.TrimSpace(args.ItemRef) == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 keyword 或 item_ref 参数"}},
				}, nil, nil
			}
			result := appServer.handleCancelFavorite(ctx, CancelFavoriteRequest{
				Keyword: args.Keyword,
				ItemRef: args.ItemRef,
			})
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "manage_collection_group",
			Description: "管理收藏分组：create|rename|delete|move",
			Annotations: &mcp.ToolAnnotations{Title: "Manage Collection Group", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("manage_collection_group", func(ctx context.Context, req *mcp.CallToolRequest, args ManageCollectionGroupArgs) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(args.Operation) == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 operation 参数"}},
				}, nil, nil
			}
			result := appServer.handleManageCollectionGroup(ctx, ManageCollectionGroupRequest{
				Operation:   args.Operation,
				GroupName:   args.GroupName,
				NewName:     args.NewName,
				ItemKeyword: args.ItemKeyword,
			})
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "list_my_items",
			Description: "读取我发布的商品列表，支持在售/已售出/下架页签",
			Annotations: &mcp.ToolAnnotations{Title: "List My Items", ReadOnlyHint: true},
		},
		withPanicRecovery("list_my_items", func(ctx context.Context, req *mcp.CallToolRequest, args ListMyItemsArgs) (*mcp.CallToolResult, any, error) {
			result := appServer.handleListMyItems(ctx, args.Tab, args.Limit)
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "edit_my_item",
			Description: "编辑我发布的商品（先定位商品，再进入编辑页填充价格/描述）",
			Annotations: &mcp.ToolAnnotations{Title: "Edit My Item", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("edit_my_item", func(ctx context.Context, req *mcp.CallToolRequest, args EditMyItemArgs) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(args.Keyword) == "" && strings.TrimSpace(args.ItemRef) == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 keyword 或 item_ref 参数"}},
				}, nil, nil
			}
			result := appServer.handleEditMyItem(ctx, EditMyItemRequest{
				Keyword:     args.Keyword,
				ItemRef:     args.ItemRef,
				Tab:         args.Tab,
				Price:       args.Price,
				Description: args.Description,
				Submit:      args.Submit,
			})
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "shelf_my_item",
			Description: "对我发布的商品执行上架/下架（action=up|down|auto）",
			Annotations: &mcp.ToolAnnotations{Title: "Shelf My Item", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("shelf_my_item", func(ctx context.Context, req *mcp.CallToolRequest, args ShelfMyItemArgs) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(args.Keyword) == "" && strings.TrimSpace(args.ItemRef) == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 keyword 或 item_ref 参数"}},
				}, nil, nil
			}
			result := appServer.handleShelfMyItem(ctx, ShelfMyItemRequest{
				Keyword: args.Keyword,
				ItemRef: args.ItemRef,
				Tab:     args.Tab,
				Action:  args.Action,
			})
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "delete_my_item",
			Description: "删除我发布的商品",
			Annotations: &mcp.ToolAnnotations{Title: "Delete My Item", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("delete_my_item", func(ctx context.Context, req *mcp.CallToolRequest, args DeleteMyItemArgs) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(args.Keyword) == "" && strings.TrimSpace(args.ItemRef) == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 keyword 或 item_ref 参数"}},
				}, nil, nil
			}
			result := appServer.handleDeleteMyItem(ctx, DeleteMyItemRequest{
				Keyword: args.Keyword,
				ItemRef: args.ItemRef,
				Tab:     args.Tab,
			})
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_item_detail",
			Description: "读取商品详情（标题、价格、卖家、想要人数、聊一聊链接、立即购买链接等）",
			Annotations: &mcp.ToolAnnotations{Title: "Get Item Detail", ReadOnlyHint: true},
		},
		withPanicRecovery("get_item_detail", func(ctx context.Context, req *mcp.CallToolRequest, args GetItemDetailArgs) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(args.ItemRef) == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 item_ref 参数"}},
				}, nil, nil
			}
			result := appServer.handleGetItemDetail(ctx, args.ItemRef)
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "favorite_item",
			Description: "触发商品收藏动作",
			Annotations: &mcp.ToolAnnotations{Title: "Favorite Item", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("favorite_item", func(ctx context.Context, req *mcp.CallToolRequest, args FavoriteItemArgs) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(args.ItemRef) == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 item_ref 参数"}},
				}, nil, nil
			}
			result := appServer.handleFavoriteItem(ctx, FavoriteItemRequest{ItemRef: args.ItemRef})
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "chat_item",
			Description: "打开商品“聊一聊”会话；可选 message 自动发送首条消息",
			Annotations: &mcp.ToolAnnotations{Title: "Chat Item", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("chat_item", func(ctx context.Context, req *mcp.CallToolRequest, args ChatItemArgs) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(args.ItemRef) == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 item_ref 参数"}},
				}, nil, nil
			}
			result := appServer.handleChatItem(ctx, ChatItemRequest{
				ItemRef: args.ItemRef,
				Message: args.Message,
			})
			return convertToMCPResult(result), nil, nil
		}),
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "buy_item",
			Description: "触发商品“立即购买”动作并返回创建订单链接",
			Annotations: &mcp.ToolAnnotations{Title: "Buy Item", DestructiveHint: boolPtr(true)},
		},
		withPanicRecovery("buy_item", func(ctx context.Context, req *mcp.CallToolRequest, args BuyItemArgs) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(args.ItemRef) == "" {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "缺少 item_ref 参数"}},
				}, nil, nil
			}
			result := appServer.handleBuyItem(ctx, BuyItemRequest{ItemRef: args.ItemRef})
			return convertToMCPResult(result), nil, nil
		}),
	)

	logrus.Info("Registered 22 MCP tools")
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
