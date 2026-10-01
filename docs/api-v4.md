# v4 协议与验证范围

本实现只请求同一管理地址下的 `/api/v4.0`，使用 `Authorization: Bearer <token>`。不保留旧版协议或自动回退。

| 用途 | REST 路径 | 方法 / 数据字段 |
|---|---|---|
| 系统实时状态 | `/monitoring/system` | GET，`results.sysinfo` |
| 接口状态与流量 | `/monitoring/interfaces-status` | GET，`results.iface_check` / `iface_stream` |
| IPv4 在线终端 | `/monitoring/clients-online` | GET，`results.total` / `data` |
| IPv6 在线终端 | `/monitoring/clients-ip6-online` | GET，`results.total` / `data` |
| IP 对象 | `/ip-objects`、`/ip-objects/{id}` | GET/POST/PUT/DELETE，`ip_total` / `ip_data`，写入 `group_name` / `group_value` |
| 域名分流 | `/routing/domain-rules`、`/routing/domain-rules/{id}` | GET/POST/PUT/DELETE，`total` / `data`，条件为 `domain.custom` / `src_addr.custom` / `time.custom` |

API 客户端同时检查 HTTP 状态和业务 `code`。`code` 非零即错误，包括 HTTP 200 的业务失败；GET 可以没有 `code`，但必须存在成功 `message`、有效 `results` 和对应字段。裸 `nil` 只在 JSON 值位置归一化为 `null`，不改写字符串。

列表按 `page`、`limit`、`order_by=id` 和 `order=asc` 翻页。总数未满足时空页、重复页或缺少字段都会失败，避免使用残缺列表进行同步。路由器上的实时列表仍可能在翻页期间变化；这里没有固件提供的事务快照保证。数字字段接受 JSON 数字或数值字符串；异常值返回采集错误。

依据是以下公开的 4.x 协议资料，而非旧版 SDK 中名为 v4 的 `/Action/call` 数据模型：

- [iKuai-Console 的 OpenAPI 目录](https://github.com/adminlby/iKuai-Console/tree/4421143de465e405dd7612d027f638449f2467c0/openapi)：文件 13（终端）、15（接口）、16（系统）、39（IP 对象）、46（域名分流）。该项目描述在 4.0.222 上使用这些 REST 接口；规范中 IP 对象最多 100 项。
- [ikuai-api v4 SDK](https://github.com/zy84338719/ikuai-api/tree/47b8fbf5ac708829ccbf4008264460906efac440)：用于交叉核对 Bearer 认证、路径和固件裸 nil 行为。本项目独立实现客户端，没有添加该 SDK 依赖。

测试使用本地 HTTP mock：认证与路径、业务错误、旧版响应拒绝、重试边界、取消、分页、4.x 嵌套写入、下载限制、同步失败边界、指标单位与去重、并发采集和服务退出。仓库中没有用户路由器凭据；目前没有进行真实 4.x 设备的读写联调，也没有把 mock 测试称为真机验证。部署前可先仅启用 exporter 核对数据，再逐项启用同步任务。
