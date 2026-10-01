# 从旧分支迁移到 v4

`master` 的旧实现与 `v4` 独立维护。本分支只支持 iKuai OS 4.x，Go 模块路径改为 `github.com/NERVEbing/ikuai-aio/v4`，所有网络方法显式接收 `context.Context`。

| 原实现 | v4 |
|---|---|
| 用户名、密码、Cookie、Login / IsLogin | API Token；不再存在登录方法 |
| `/Action/login` / `/Action/call` | `/api/v4.0` REST |
| `IKUAI_USERNAME` / `IKUAI_PASSWORD` | `IKUAI_TOKEN` 或 `IKUAI_TOKEN_FILE` |
| `IKUAI_CRON_CUSTOM_ISP_<n>` 的 `|` 字符串 | `IKUAI_IP_OBJECT_<n>` 的 JSON；管理 4.x IP 对象 |
| `IKUAI_CRON_STREAM_DOMAIN_<n>` 的 `|` 字符串 | `IKUAI_DOMAIN_RULE_<n>` 的 JSON；按策略名称更新 |
| 删除后重建整个列表 | 更新既有资源 ID；IP 分组确认写入成功后清理多余项 |
| 内存 KB、流量总量 Gauge、旧指标名称 | 内存字节、流量 Counter、v4 指标与新 Grafana 面板 |
| 配置单例、客户端读取进程环境 | 配置一次加载，再向客户端 / 采集器 / 任务注入 |
| 不可取消的后台服务 | 信号取消、任务截止时间和 HTTP graceful shutdown |

迁移步骤：创建 4.x Token，复制 `.env.example` 并填写配置；将任务改为 JSON 格式；导入仓库内的 Grafana 面板；构建并启动 v4。旧账号或任务变量会使启动失败并指出待替换的设置。

IP 对象使用 `<前缀>AIO0001` 等保留名称。不要在其他任务或手工对象中使用该命名空间。将所需对象引用到 4.x 策略中；这不会自动迁移旧版自定义运营商引用，也不会自动给其他策略加入新分组。

域名任务的 `name` 精确对应 `tagname`。任务完整管理这个策略的域名、源地址、线路、优先级、启用状态和全天时间条件，不适合共用一条手工维护的同名策略。多个任务不能拥有相同资源名称。

IP 对象列表变长或变短时会保持仍存在分组的 ID，但排序变化可能把某个 IP 移入另一个分组；策略如果需要整张列表，应引用全部分组。多个对象的协调不是原子操作，业务失败或断网可能造成部分更新，下次运行将继续协调。
