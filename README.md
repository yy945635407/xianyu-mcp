# xianyu-mcp

基于 Go + Gin + MCP SDK + Rod 的闲鱼 MCP 服务（首版已实现登录相关能力）。

AI Agent 专用说明见：`AI_USAGE.md`

## 已实现

- `check_login_status`：检查闲鱼登录状态
- `get_login_qrcode`：获取登录二维码
- `delete_cookies`：清理 cookies 重置登录
- `search_items`：按关键词搜索商品摘要
- `list_conversations`：读取消息会话列表（用户名、最新消息、状态）
- `get_messages`：按用户名查询消息（含商品上下文与订单状态）
- `send_message`：按用户名发送消息（支持幂等 `client_msg_id`、重试和会话状态拦截）
- `pull_im_events`：按游标拉取 IM 增量事件流
- `get_im_session_state` / `set_im_session_state` / `mark_im_session_read`：会话状态与已读管理
- `upsert_im_knowledge` / `list_im_knowledge` / `match_im_knowledge` / `delete_im_knowledge`：智能客服知识库管理与匹配
- `publish_item`：按网页字段发布闲置（支持 `submit=false` 仅填表校验）
- `list_orders`：读取订单列表（可按页签筛选）
- `remind_ship`：提醒卖家发货（买家订单场景）
- `ship_order`：触发去发货（若网页端受限会返回需 APP）
- HTTP API 与 MCP Streamable HTTP 双入口

## 快速开始

```bash
go run ./cmd/login
```

手动登录成功后，再启动服务：

```bash
go run . -headless=false -port=:18061
```

MCP 地址：`http://localhost:18061/mcp`

搜索接口示例：

```bash
curl 'http://localhost:18061/api/v1/search?keyword=iphone&limit=5'
```

消息接口示例：

```bash
# 读取会话列表
curl 'http://localhost:18061/api/v1/im/conversations?limit=20'

# 查询某个用户名会话
curl 'http://localhost:18061/api/v1/im/messages?username=发韧的树枝&limit=30'

# 发送消息
curl -X POST 'http://localhost:18061/api/v1/im/send' \
  -H 'Content-Type: application/json' \
  -d '{"username":"发韧的树枝","message":"你好","limit":20,"client_msg_id":"msg-001","max_retries":2}'

# 增量事件流
curl 'http://localhost:18061/api/v1/im/events?since_id=0&limit=50'

# 会话状态
curl -X POST 'http://localhost:18061/api/v1/im/session/state' \
  -H 'Content-Type: application/json' \
  -d '{"username":"发韧的树枝","mode":"human","handoff_reason":"人工接管"}'

# 知识库 upsert
curl -X POST 'http://localhost:18061/api/v1/im/kb/upsert' \
  -H 'Content-Type: application/json' \
  -d '{
    "title":"价格解释",
    "keywords":["最低多少","还能便宜吗","包邮吗"],
    "answer":"你好，这个价格已经是活动价，支持包邮。",
    "order_statuses":["未下单","已拍下"],
    "priority":10
  }'

# 知识库匹配
curl -X POST 'http://localhost:18061/api/v1/im/kb/match' \
  -H 'Content-Type: application/json' \
  -d '{"message":"还能便宜吗？包邮吗","top_k":3}'
```

发布接口示例（不真正提交）：

```bash
curl -X POST 'http://localhost:18061/api/v1/publish/item' \
  -H 'Content-Type: application/json' \
  -d '{
    "images": ["/abs/path/a.jpg"],
    "description": "九成新，正常使用",
    "price": "88",
    "original_price": "120",
    "shipping_type": "包邮",
    "shipping_fee": "",
    "support_self_pickup": false,
    "location_keyword": "成都",
    "spec_types": [],
    "submit": false
  }'
```

订单接口示例：

```bash
# 列订单
curl 'http://localhost:18061/api/v1/orders/list?tab=全部&limit=20'

# 提醒发货
curl -X POST 'http://localhost:18061/api/v1/orders/remind_ship' \
  -H 'Content-Type: application/json' \
  -d '{"order_keyword":"车展门票","seller_name":"后晋登山的大挣"}'

# 去发货（卖家会话）
curl -X POST 'http://localhost:18061/api/v1/orders/ship' \
  -H 'Content-Type: application/json' \
  -d '{"username":"发韧的树枝"}'
```

## 浏览器探索调试

使用可见浏览器抓取当前登录态下的关键入口和候选能力：

```bash
go run ./cmd/explore
```

## 已验证可开发方向

- 发布闲置（`/publish`）
- 搜索商品与筛选（`/search`）
- 我的闲鱼页数据（`/personal`）
- 会话消息读取（`/im`）
- 订单查询（`/bought`）
- 收藏夹读取（`/collection`）
- 账号与安全信息读取（`/account`）
- 商品详情与动作（聊一聊/立即购买/收藏）
