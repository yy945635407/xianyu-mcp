# xianyu-mcp

基于 Go + Gin + MCP SDK + Rod 的闲鱼 MCP 服务（首版已实现登录相关能力）。

## 已实现

- `check_login_status`：检查闲鱼登录状态
- `get_login_qrcode`：获取登录二维码
- `delete_cookies`：清理 cookies 重置登录
- `search_items`：按关键词搜索商品摘要
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
