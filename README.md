# xianyu-mcp

基于 Go + Gin + MCP SDK + Rod 的闲鱼 MCP 服务（首版已实现登录相关能力）。

## 已实现

- `check_login_status`：检查闲鱼登录状态
- `get_login_qrcode`：获取登录二维码
- `delete_cookies`：清理 cookies 重置登录
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
