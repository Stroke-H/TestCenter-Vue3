# TTmins 日志

TTmins 日志将 `Chii-Logs-macOS` 的远程调试服务集成到 TestCenter，不再启动独立 Node.js 服务。服务端连接管理、WebSocket 中继和受限资源代理均由 Go 实现；手机端调试脚本和 DevTools 静态资源保留上游受控构建。

## 使用

1. 从仪表盘的“API 工具”打开“TTmins日志”。
2. 复制页面中的 `Client script URL`。页面会优先选择当前 Mac 的局域网 IPv4，即使平台是从 `localhost` 打开，也不会将 `localhost` 复制给手机。存在多个网络地址时可从右侧列表切换。
3. 在 TikTok 小程序测试包的日志设置中粘贴地址，先点击 `Save address`，再点击 `Enable`。
4. 设备出现在列表后，点击 `Inspect` 打开 Console、Network 等调试面板。

测试包必须允许当前平台来源。平台域名、IP 或端口变更后，需同步检查测试包的 `trustedDomains`。

## 边界与安全

- 设备列表、Inspect 会话创建和断开操作需要 TestCenter 登录态。
- Inspect 使用两分钟内有效的一次性随机凭证，凭证成功使用后立即失效。
- 资源代理只允许访问当前在线调试页面的精确协议和主机，且不转发 Cookie 或 Authorization。
- 日志通过 WebSocket 实时中继，不写入数据库或本地文件。后端重启后已有连接需重新启用。
- 服务启动时会校验 `target.js` 的 SHA-384 SRI 与协议版本，不匹配时拒绝启动。

## 受控客户端

- 版本：`1.0.0`
- 协议：`1`
- SRI：`sha384-vj0BeQRVPMh9eJb7I8B/2l6MOWTBQLJ6DMzXCxSUGGMBDCkvcjRwW38blWie5UI7`
- DevTools 资源：Chii `1.15.5`

Chii 许可证保留在 `server/services/assets/ttmins_logs/CHII-LICENSE`。
