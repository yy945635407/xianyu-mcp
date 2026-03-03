# xianyu-mcp

基于 **Go + Gin + MCP SDK + Rod** 的闲鱼自动化服务，提供：
- 标准 HTTP API（`/api/v1/*`）
- MCP Streamable HTTP（`/mcp`）

项目目标是把闲鱼网页操作能力封装成可编排的 API / MCP Tools，便于 AI Agent 或业务系统直接调用。

AI Agent 专用说明见：[AI_USAGE.md](./AI_USAGE.md)

## 1. 软件架构

### 1.1 分层架构

```text
            Client / Agent
         (HTTP or MCP Client)
                    |
         +----------+----------+
         |   Gin Router        |
         | /health /api/v1 /mcp|
         +----------+----------+
                    |
             AppServer/Handlers
                    |
              XianyuService
          +---------+---------+
          |                   |
    Local JSON Stores    Rod Browser Session
     (状态/幂等/知识)       (单例复用浏览器)
          |                   |
          +---------+---------+
                    |
            xianyu/* Action
       (登录/消息/订单/发布等网页自动化)
```

### 1.2 模块职责

| 层 | 关键文件 | 职责 |
|---|---|---|
| 入口层 | `main.go`, `app_server.go` | 解析启动参数、初始化服务、优雅退出 |
| 路由层 | `routes.go`, `middleware.go` | 注册 HTTP/MCP 路由、CORS、panic 恢复 |
| 协议适配层 | `handlers_api.go`, `mcp_handlers.go`, `mcp_server.go` | HTTP/MCP 参数绑定、调用服务层、统一返回格式 |
| 业务编排层 | `service.go` | 参数兜底、重试、幂等、会话策略、知识匹配、自动上下文 |
| 自动化动作层 | `xianyu/*.go` | 具体网页操作（登录、消息、发布、订单、收藏、社区等） |
| 状态持久层 | `im_*_store.go`, `storage_paths.go` | 本地 JSON 存储事件、会话状态、发送记录、知识库 |
| 浏览器封装层 | `browser/browser.go`, `configs/*`, `cookies/*` | 浏览器实例配置、代理、Cookie 读写、无头模式控制 |

### 1.3 关键设计点

1. 双协议入口
- HTTP 供业务系统直接接入。
- MCP 供 Agent 通过 Tool 调用。

2. 浏览器单例复用
- `service.go` 内通过 `sharedBrowserInst + refCount` 复用浏览器，减少频繁启动开销。

3. 本地状态持久化
- 事件流、会话状态、消息发送幂等记录、知识库都落盘到 `data/*.json`（可通过 `DATA_DIR` 改目录）。

4. 消息发送安全控制
- 发送前检查会话模式（`bot/human`）和会话锁。
- `client_msg_id` 做幂等，重复请求不会重复发送。

5. 增量事件流
- 从会话列表生成增量事件（`since_id` 游标）并支持阻塞等待接口 `wait_im_events`。

6. 知识库上下文匹配
- 支持关键词 + 商品上下文 + 订单状态匹配。
- 支持 `auto_context` 按用户名自动补全上下文（含缓存与刷新节流）。

## 2. 能力总览

当前已注册 **40 个 MCP Tools**，HTTP 侧提供同等能力。

### 2.1 登录与基础
- `check_login_status`
- `get_login_qrcode`
- `delete_cookies`
- `search_items`

### 2.2 消息与客服自动化
- `list_conversations`
- `get_messages`
- `pull_im_events`
- `wait_im_events`
- `get_im_session_state`
- `list_im_session_states`
- `set_im_session_state`
- `mark_im_session_read`
- `send_message`
- `upsert_im_knowledge`
- `list_im_knowledge`
- `delete_im_knowledge`
- `match_im_knowledge`

### 2.3 发布与订单
- `publish_item`
- `list_orders`
- `remind_ship`
- `ship_order`
- `ship_with_logistics`
- `confirm_receipt`
- `review_order`
- `handle_refund`

### 2.4 收藏、我的商品、商品详情
- `list_collections`
- `cancel_favorite`
- `manage_collection_group`
- `list_my_items`
- `edit_my_item`
- `shelf_my_item`
- `delete_my_item`
- `get_item_detail`
- `favorite_item`
- `chat_item`
- `buy_item`

### 2.5 账号与社区
- `get_account_security`
- `get_community_feed`
- `interact_community`
- `get_customer_service`
- `open_customer_service`

## 3. 目录结构

```text
xianyu-mcp/
├── main.go                  # 服务启动入口
├── app_server.go            # HTTP Server 生命周期
├── routes.go                # 路由注册
├── handlers_api.go          # HTTP 处理器
├── mcp_server.go            # MCP Tool 注册
├── mcp_handlers.go          # MCP 处理器
├── service.go               # 核心业务编排
├── im_event_store.go        # 增量事件存储
├── im_session_store.go      # 会话状态存储
├── im_send_record_store.go  # 消息幂等记录存储
├── im_knowledge_store.go    # 知识库存储
├── storage_paths.go         # DATA_DIR 路径管理
├── browser/                 # 浏览器初始化
├── cookies/                 # Cookie 读写
├── configs/                 # 运行配置
├── cmd/login/               # 手动登录工具
├── cmd/explore/             # 页面入口探索工具
├── xianyu/                  # 闲鱼动作实现层
├── AI_USAGE.md              # AI/Agent 调用指南
└── README.md
```

## 4. 快速开始

### 4.1 环境要求
- Go `1.24+`
- 本机可用 Chrome/Chromium
- 可访问 `https://www.goofish.com/`

### 4.2 安装依赖

```bash
go mod tidy
```

### 4.3 首次登录（必须）

```bash
go run ./cmd/login
```

说明：
- 会启动可见浏览器（非 headless）。
- 扫码后会将 Cookie 保存到 `cookies.json`（或 `COOKIES_PATH` 指定路径）。

### 4.4 启动服务

```bash
go run . -headless=false -port=:18061
```

启动后可用地址：
- 健康检查：`GET http://127.0.0.1:18061/health`
- HTTP API：`http://127.0.0.1:18061/api/v1`
- MCP：`http://127.0.0.1:18061/mcp`

## 5. 配置说明

### 5.1 启动参数

| 参数 | 默认值 | 说明 |
|---|---|---|
| `-headless` | `true` | 是否无头模式运行浏览器 |
| `-bin` | `""` | 浏览器二进制路径（优先于自动查找） |
| `-port` | `:18061` | HTTP 服务监听地址 |

### 5.2 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `ROD_BROWSER_BIN` | 空 | 浏览器路径（等价于 `-bin`） |
| `XIANYU_PROXY` | 空 | 浏览器代理，如 `http://user:pass@host:port` |
| `COOKIES_PATH` | `cookies.json` | Cookie 文件路径 |
| `DATA_DIR` | `data` | 本地状态存储目录 |
| `XIANYU_IM_SCAN_INTERVAL_MS` | `15000` | IM 轮询最小间隔（毫秒，最小 1000） |
| `XIANYU_AUTOCTX_REFRESH_SEC` | `180` | 自动上下文刷新间隔（秒，最小 30） |
| `XIANYU_AUTOCTX_TTL_SEC` | `900` | 自动上下文缓存 TTL（秒，最小 60） |

## 6. 本地数据文件

默认在 `data/` 目录下生成：

| 文件 | 用途 |
|---|---|
| `im_events.json` | IM 增量事件流（`since_id` 游标） |
| `im_session_states.json` | 会话模式、人工接管、锁、已读时间 |
| `im_send_records.json` | `client_msg_id` 幂等记录 |
| `im_knowledge.json` | 智能客服知识库 |

## 7. HTTP 使用方法与示例

### 7.1 返回格式

成功：

```json
{
  "success": true,
  "data": {},
  "message": "..."
}
```

失败：

```json
{
  "error": "...",
  "code": "...",
  "details": "..."
}
```

### 7.2 基础验证

```bash
# 健康检查
curl 'http://127.0.0.1:18061/health'

# 登录状态
curl 'http://127.0.0.1:18061/api/v1/login/status'

# 获取二维码（未登录时）
curl 'http://127.0.0.1:18061/api/v1/login/qrcode'

# 重置登录态
curl -X DELETE 'http://127.0.0.1:18061/api/v1/login/cookies'
```

### 7.3 搜索商品

```bash
curl 'http://127.0.0.1:18061/api/v1/search?keyword=iphone&limit=5'
```

### 7.4 IM 自动回复闭环（推荐顺序）

1. 拉取增量事件

```bash
curl 'http://127.0.0.1:18061/api/v1/im/events?since_id=0&limit=50&scan_limit=30'
```

2. 阻塞等待增量（守护进程推荐）

```bash
curl 'http://127.0.0.1:18061/api/v1/im/events/wait?since_id=0&limit=50&scan_limit=30&timeout_sec=30&poll_ms=1200'
```

3. 查看会话消息

```bash
curl 'http://127.0.0.1:18061/api/v1/im/messages?username=发韧的树枝&limit=30'
```

4. 查询会话状态（防误发）

```bash
curl 'http://127.0.0.1:18061/api/v1/im/session/state?username=发韧的树枝'
```

5. 发送消息（带幂等 ID）

```bash
curl -X POST 'http://127.0.0.1:18061/api/v1/im/send' \
  -H 'Content-Type: application/json' \
  -d '{
    "username":"发韧的树枝",
    "message":"你好，这边已收到",
    "client_msg_id":"auto-10001-a1b2c3d4",
    "max_retries":2,
    "force":false,
    "limit":20
  }'
```

6. 标记会话已读

```bash
curl -X POST 'http://127.0.0.1:18061/api/v1/im/session/mark_read' \
  -H 'Content-Type: application/json' \
  -d '{"username":"发韧的树枝","limit":30}'
```

### 7.5 知识库管理与匹配

```bash
# 新增/更新知识
curl -X POST 'http://127.0.0.1:18061/api/v1/im/kb/upsert' \
  -H 'Content-Type: application/json' \
  -d '{
    "title":"价格解释",
    "keywords":["最低多少","还能便宜吗","包邮吗"],
    "answer":"你好，这个价格已经是活动价，支持包邮。",
    "order_statuses":["未下单","已拍下"],
    "priority":10
  }'

# 列表查询
curl 'http://127.0.0.1:18061/api/v1/im/kb/list?query=便宜&limit=20'

# 匹配知识（支持 auto_context）
curl -X POST 'http://127.0.0.1:18061/api/v1/im/kb/match' \
  -H 'Content-Type: application/json' \
  -d '{
    "message":"还能便宜吗？包邮吗",
    "username":"发韧的树枝",
    "top_k":3,
    "auto_context":true
  }'
```

### 7.6 发布闲置

```bash
# 仅填表校验（不提交）
curl -X POST 'http://127.0.0.1:18061/api/v1/publish/item' \
  -H 'Content-Type: application/json' \
  -d '{
    "images": ["/abs/path/a.jpg"],
    "description": "九成新，正常使用",
    "price": "88",
    "original_price": "120",
    "shipping_type": "包邮",
    "location_keyword": "成都",
    "submit": false
  }'
```

> 建议先 `submit=false` 验证字段与页面元素，再改为 `submit=true` 真正发布。

### 7.7 订单能力

```bash
# 查询订单
curl 'http://127.0.0.1:18061/api/v1/orders/list?tab=全部&limit=20'

# 提醒卖家发货
curl -X POST 'http://127.0.0.1:18061/api/v1/orders/remind_ship' \
  -H 'Content-Type: application/json' \
  -d '{"order_keyword":"车展门票","seller_name":"后晋登山的大挣"}'

# 会话触发去发货
curl -X POST 'http://127.0.0.1:18061/api/v1/orders/ship' \
  -H 'Content-Type: application/json' \
  -d '{"username":"发韧的树枝"}'

# 带物流发货
curl -X POST 'http://127.0.0.1:18061/api/v1/orders/ship_with_logistics' \
  -H 'Content-Type: application/json' \
  -d '{"username":"发韧的树枝","company":"中通","tracking_no":"YT123456789"}'

# 确认收货
curl -X POST 'http://127.0.0.1:18061/api/v1/orders/confirm_receipt' \
  -H 'Content-Type: application/json' \
  -d '{"order_keyword":"耳机"}'

# 评价订单
curl -X POST 'http://127.0.0.1:18061/api/v1/orders/review' \
  -H 'Content-Type: application/json' \
  -d '{"order_keyword":"耳机","score":5,"content":"发货快，描述一致"}'

# 退款处理
curl -X POST 'http://127.0.0.1:18061/api/v1/orders/refund' \
  -H 'Content-Type: application/json' \
  -d '{"order_keyword":"耳机","action":"detail"}'
```

### 7.8 收藏、我的商品、商品详情

```bash
# 收藏夹列表
curl 'http://127.0.0.1:18061/api/v1/collections/list?limit=20'

# 取消收藏（关键词或 item_ref 二选一）
curl -X POST 'http://127.0.0.1:18061/api/v1/collections/cancel' \
  -H 'Content-Type: application/json' \
  -d '{"keyword":"iPhone 13"}'

# 我的商品列表
curl 'http://127.0.0.1:18061/api/v1/my/items?tab=在售&limit=20'

# 商品详情
curl 'http://127.0.0.1:18061/api/v1/item/detail?item_ref=https://www.goofish.com/item?id=xxxxxxxx'

# 商品聊一聊
curl -X POST 'http://127.0.0.1:18061/api/v1/item/chat' \
  -H 'Content-Type: application/json' \
  -d '{"item_ref":"https://www.goofish.com/item?id=xxxxxxxx","message":"你好，还在吗"}'
```

## 8. MCP 接入说明

### 8.1 MCP 地址
- `http://127.0.0.1:18061/mcp`

### 8.2 说明
- 基于 `go-sdk` 的 **Streamable HTTP**。
- Tool 输出主体通常在 `content[].text`，多数是 JSON 字符串（客户端需再解析一次）。
- 建议调用策略见 [AI_USAGE.md](./AI_USAGE.md)（已包含消息自动回复最小闭环与风控建议）。

## 9. HTTP 路由清单

### 9.1 基础
- `GET /health`
- `GET /api/v1/login/status`
- `GET /api/v1/login/qrcode`
- `DELETE /api/v1/login/cookies`

### 9.2 搜索
- `GET|POST /api/v1/search`

### 9.3 IM / 客服
- `GET /api/v1/im/conversations`
- `GET|POST /api/v1/im/messages`
- `GET|POST /api/v1/im/events`
- `GET|POST /api/v1/im/events/wait`
- `GET /api/v1/im/session/state`
- `GET /api/v1/im/session/states`
- `POST /api/v1/im/session/state`
- `POST /api/v1/im/session/mark_read`
- `POST /api/v1/im/send`
- `GET /api/v1/im/kb/list`
- `POST /api/v1/im/kb/upsert`
- `POST /api/v1/im/kb/delete`
- `POST /api/v1/im/kb/match`

### 9.4 发布与订单
- `POST /api/v1/publish/item`
- `GET /api/v1/orders/list`
- `POST /api/v1/orders/remind_ship`
- `POST /api/v1/orders/ship`
- `POST /api/v1/orders/ship_with_logistics`
- `POST /api/v1/orders/confirm_receipt`
- `POST /api/v1/orders/review`
- `POST /api/v1/orders/refund`

### 9.5 收藏 / 我的商品 / 商品详情
- `GET /api/v1/collections/list`
- `POST /api/v1/collections/cancel`
- `POST /api/v1/collections/groups/manage`
- `GET /api/v1/my/items`
- `POST /api/v1/my/items/edit`
- `POST /api/v1/my/items/shelf`
- `POST /api/v1/my/items/delete`
- `GET /api/v1/item/detail`
- `POST /api/v1/item/favorite`
- `POST /api/v1/item/chat`
- `POST /api/v1/item/buy`

### 9.6 账号与社区
- `GET /api/v1/account/security`
- `GET /api/v1/community/feed`
- `POST /api/v1/community/interact`
- `GET /api/v1/customer/service`
- `POST /api/v1/customer/open`

## 10. 调试与排障

### 10.1 页面入口探索

```bash
go run ./cmd/explore
```

用于快速扫描当前登录态下可见入口，辅助扩展新能力。

### 10.2 常见问题

1. `conversation not found`
- 先 `list_conversations` 核对 `username`，避免昵称不一致。

2. `UI not ready`
- 页面结构抖动导致元素未就绪，可重试并降低并发。

3. `client_msg_id already used with different payload`
- 同一个 `client_msg_id` 发送了不同消息体，需更换 ID。

4. 物流发货返回 `requires_app=true`
- 当前网页端能力受限，需转闲鱼 App 完成。

5. 端口冲突
- 改端口启动：`go run . -port=:18062`

## 11. 安全建议

- 当前服务默认无鉴权、CORS 允许 `*`，请仅在受信内网使用。
- 不建议直接暴露公网；如需对外，请在网关层增加鉴权与访问控制。
- 生产使用建议配合审计日志、调用限流和人工兜底策略。
