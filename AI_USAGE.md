# xianyu-mcp AI 使用说明

这份文档给 AI Agent（如 OpenClaw、Cursor Agent、MCP Client）使用，目标是让模型能稳定调用 `xianyu-mcp` 完成闲鱼自动化任务。

## 1. 服务入口

- HTTP API 基地址：`http://127.0.0.1:18061/api/v1`
- MCP 地址：`http://127.0.0.1:18061/mcp`
- 健康检查：`GET http://127.0.0.1:18061/health`

启动命令：

```bash
# 首次先登录（会打开可见浏览器）
go run ./cmd/login

# 启动服务（调试建议可见浏览器）
go run . -headless=false -port=:18061
```

可用环境变量：

- `ROD_BROWSER_BIN`：浏览器可执行路径
- `XIANYU_PROXY`：浏览器代理（如 `http://user:pass@host:port`）

## 2. 返回格式约定

### HTTP

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

### MCP

- 每个 tool 的返回主体通常在 `content[].text`
- 多数 tool 的 `text` 是 JSON 字符串（需要再 parse 一层）
- 报错时通常 `isError=true`，`text` 是中文错误描述

## 3. 工具清单（40个）

### 3.1 登录与基础

- `check_login_status`
- `get_login_qrcode`
- `delete_cookies`
- `search_items`（必填：`keyword`）

### 3.2 消息与客服

- `list_conversations`
- `get_messages`（必填：`username`）
- `pull_im_events`
- `get_im_session_state`（必填：`username`）
- `list_im_session_states`
- `set_im_session_state`（必填：`username`）
- `mark_im_session_read`（必填：`username`）
- `send_message`（必填：`username`, `message`）
- `upsert_im_knowledge`（必填：`keywords`, `answer`）
- `list_im_knowledge`
- `delete_im_knowledge`（必填：`id`）
- `match_im_knowledge`（必填：`message`）

### 3.3 发布与订单

- `publish_item`（必填：`images`, `description`, `price`）
- `list_orders`
- `remind_ship`
- `ship_order`（必填：`username`）
- `ship_with_logistics`（必填：`username`, `tracking_no`）
- `confirm_receipt`
- `review_order`
- `handle_refund`

### 3.4 收藏、我的商品、商品详情

- `list_collections`
- `cancel_favorite`（`keyword`/`item_ref` 二选一）
- `manage_collection_group`（必填：`operation`）
- `list_my_items`
- `edit_my_item`（`keyword`/`item_ref` 二选一）
- `shelf_my_item`（`keyword`/`item_ref` 二选一）
- `delete_my_item`（`keyword`/`item_ref` 二选一）
- `get_item_detail`（必填：`item_ref`）
- `favorite_item`（必填：`item_ref`）
- `chat_item`（必填：`item_ref`）
- `buy_item`（必填：`item_ref`）

### 3.5 账号与社区

- `get_account_security`
- `get_community_feed`
- `interact_community`（必填：`keyword`）
- `get_customer_service`
- `open_customer_service`

## 4. AI 调用规则（强建议）

### 4.1 登录前置

1. 先调用 `check_login_status`
2. 未登录则调用 `get_login_qrcode` 并等待人工扫码
3. 重新调用 `check_login_status`，登录成功后再执行业务

### 4.2 消息自动回复最小闭环

推荐顺序：

1. `pull_im_events(since_id, limit=50, scan_limit=30)` 拉增量事件
2. 对每个 `event.username` 调 `get_im_session_state`
3. 若 `mode=human` 或会话锁生效，跳过发送
4. 调 `get_messages(username, limit=30)` 拿上下文
5. 调 `match_im_knowledge(message, username, item_ref, order_status, top_k=3, auto_context=true)`
6. 命中后调 `send_message`
7. 成功后调 `mark_im_session_read(username)`
8. 推进本地 cursor（使用 `next_cursor`）

### 4.3 `send_message` 幂等和风控

发送参数建议：

```json
{
  "username": "xxx",
  "message": "yyy",
  "client_msg_id": "auto-{event_id}-{hash(username)}",
  "max_retries": 2,
  "force": false,
  "limit": 20
}
```

关键返回字段解释：

- `blocked=true`：被会话策略阻止（如人工接管），不要重试
- `deduplicated=true`：命中幂等记录，视为成功
- `attempts`：实际尝试次数

### 4.4 会话状态控制

- `set_im_session_state` 支持：
- `mode`：`bot|human`
- `handoff_reason`：人工接管原因
- `lock_seconds` + `lock_owner`：临时锁会话
- `clear_lock=true`：清除会话锁

## 5. 关键参数语义

- 订单状态规范：`未下单`、`已拍下`、`我已发货`、`已收货`
- 知识库条目支持上下文过滤：
- `item_ref`：商品维度过滤
- `order_statuses`：订单状态过滤
- `priority`：优先级，越大越优先
- `enabled`：是否启用

## 6. HTTP 示例（AI可直接复用）

### 6.1 拉消息增量

```bash
curl 'http://127.0.0.1:18061/api/v1/im/events?since_id=0&limit=50&scan_limit=30'
```

### 6.2 发送消息

```bash
curl -X POST 'http://127.0.0.1:18061/api/v1/im/send' \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "发韧的树枝",
    "message": "你好，这边已收到",
    "client_msg_id": "auto-10001-a1b2c3d4",
    "max_retries": 2,
    "force": false,
    "limit": 20
  }'
```

### 6.3 知识库匹配

```bash
curl -X POST 'http://127.0.0.1:18061/api/v1/im/kb/match' \
  -H 'Content-Type: application/json' \
  -d '{
    "message": "还能便宜点吗",
    "username": "发韧的树枝",
    "top_k": 3,
    "auto_context": true
  }'
```

## 7. 常见失败与处理

- `conversation not found`：用户会话不在列表，先 `list_conversations` 确认用户名
- `UI not ready`：网页端波动，稍后重试或降低并发
- `client_msg_id already used with different payload`：同 ID 被不同消息复用，改用新 ID
- `blocked=true`：会话在 `human` 或锁定，需先 `set_im_session_state` 切回 `bot`
- 端口冲突：改端口启动，如 `-port=:18062`

## 8. 给 Agent 的最小执行策略

建议固定策略：

1. 每轮只处理新事件（基于 cursor）
2. 只对入站用户文本触发回复
3. 每个事件最多发送一次（事件去重 + `client_msg_id` 幂等）
4. 发送失败累计阈值后切 `human`，防止失控
5. 全流程写审计日志，便于回放

