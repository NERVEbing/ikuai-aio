# iKuai All in One · v4

面向 **iKuai OS 4.x** 的 Prometheus Exporter、IP 对象同步和域名分流同步服务。使用 `/api/v4.0` REST API 与 Bearer Token。这个分支没有 3.x 登录、Cookie 或 `/Action/call` 协议，也不接受旧任务配置。

## 快速启动

1. 在 4.x 路由器的 API 管理界面创建 Token，授权系统、接口、IPv4/IPv6 终端监控读取权限。需要同步列表时，再授权 IP 对象或域名分流策略的读取和修改权限。
2. 克隆 `v4`，复制配置并填写路由地址和 Token：

```sh
git clone --branch v4 https://github.com/NERVEbing/ikuai-aio.git
cd ikuai-aio
cp .env.example .env
# 编辑 .env 中的 IKUAI_ADDR 与 IKUAI_TOKEN
docker compose -f deploy/docker-compose.yml up -d --build
```

Compose 从当前源码构建 `v4` 镜像。`/metrics` 默认在 `http://localhost:8000/metrics`，`/healthz` 是进程存活检查，不会向路由器发请求。路由器不可达时服务保持运行，指标会报告失败。

```yaml
# Prometheus scrape_configs 示例
- job_name: ikuai
  scrape_interval: 30s
  scrape_timeout: 20s
  static_configs:
    - targets: ['ikuai-aio:8000']
```

将 [grafana/iKuai.json](grafana/iKuai.json) 导入 Grafana，选择 Prometheus 数据源。它使用 v4 指标；原面板 ID `19247` 不对应本分支的指标契约。

## 配置

| 环境变量 | 默认值 | 用途 |
|---|---|---|
| `IKUAI_ADDR` | `http://192.168.1.1` | 路由器管理地址，支持反向代理路径前缀 |
| `IKUAI_TOKEN` | 必填 | 4.x API Token |
| `IKUAI_TOKEN_FILE` | 空 | 从文件读取 Token，与 `IKUAI_TOKEN` 互斥；容器用户必须能读此文件 |
| `HTTP_TIMEOUT` | `10s` | 单个 HTTP 请求的总预算，包含读取及重试等待 |
| `HTTP_READ_RETRIES` | `2` | GET 最多重试次数，范围 0–5；写入请求不重试 |
| `HTTP_INSECURE_SKIP_VERIFY` | `false` | 仅对路由器显式关闭 TLS 证书校验 |
| `TZ` | `Asia/Shanghai` | 任务时区，时区数据已编入二进制 |
| `IKUAI_EXPORTER_LISTEN_ADDR` | `0.0.0.0:8000` | HTTP 监听地址 |
| `IKUAI_EXPORTER_DISABLE` | `false` | 禁用 `/metrics`，保留 `/healthz` 和任务调度 |
| `IKUAI_COLLECT_IPV6` | `true` | 是否采集 IPv6 在线终端 |
| `IKUAI_SCRAPE_TIMEOUT` | `15s` | 一次采集全部接口和分页的总预算 |
| `IKUAI_JOB_TIMEOUT` | `5m` | 单个同步任务的总预算 |
| `IKUAI_CRON_SKIP_START` | `false` | 跳过启动时的首次同步 |
| `IKUAI_IP_OBJECT_<数字>` | 空 | JSON 格式 IP 对象任务 |
| `IKUAI_DOMAIN_RULE_<数字>` | 空 | JSON 格式域名分流任务 |

配置在启动时完整校验。JSON 字段拼写错误、重复名称、无效地址、时区或周期会返回错误。Token 文件只在启动时读取，更新后需重启服务。API 认证过期会报告错误，不会尝试旧版登录。

### IP 对象同步

```dotenv
IKUAI_IP_OBJECT_1='{"schedule":"8h","name":"China","urls":["https://raw.githubusercontent.com/Hackl0us/GeoIP2-CN/release/CN-ip-cidr.txt"],"comment":"ikuai-aio"}'
```

`name` 是 1–8 字的中文、ASCII 字母或数字前缀。每个 IP 对象最多 100 项，服务生成 `ChinaAIO0001`、`ChinaAIO0002` 等名称；这个命名空间由对应任务管理。允许单个 IPv4、CIDR 和 IPv4 范围。来源按行读取，支持空行、BOM、`#`、`;`、`//` 注释，规范化、去重后排序。

这些是 **4.x IP 对象**，并非旧版自定义运营商记录。请在 4.x 策略中引用所需对象。同步新增分组时，不会自动修改其他策略的对象引用。缩短列表后，仍被其他策略引用的旧分组可能无法删除，任务会明确报告该错误。

### 域名分流同步

```dotenv
IKUAI_DOMAIN_RULE_1='{"schedule":"10m","name":"GFW","interface":"wan2","urls":["https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/gfw.txt"],"source":["192.168.1.10-192.168.1.20"],"priority":31,"comment":"ikuai-aio"}'
```

按 `name` 精确查找策略；已有策略通过 PUT 原位更新，缺少时创建。名称最多 15 字，支持中文、ASCII 字母、数字、下划线和连字符，首字不能是符号。每个任务指定一条 `interface`；多线路请配置多个不同名称的任务。`source` 可省略，表示不限制源地址；`priority` 默认为 31，范围 0–63。域名来源须为按行排列的 ASCII/punycode 主机名，不接受 URL、通配符或其他规则文件格式。任务将策略设为启用、每天全天生效，并完整管理该策略的条件。

`schedule` 支持至少 1 秒的 Go duration（如 `8h`）或五字段 cron（如 `0 6 * * *`）。默认在启动时先同步一次；同一任务的重叠触发会跳过，各任务的路由器写入串行执行。

**同步边界：** 每个下载源都必须成功且非空，所有条目验证后才开始修改路由器。IP 对象优先创建缺失分组、更新已有 ID，全部成功后才删除多余分组。多资源更新不是事务，途中失败可能留下已创建或更新的分组；下一次任务会继续协调。网络失败后的写入不会自动重放。

## 指标

| 指标 | 类型 / 单位 |
|---|---|
| `ikuai_info` | 固件版本、架构和主机信息 |
| `ikuai_up{id="host"}` | 系统接口可用性；失败时始终为 0 |
| `ikuai_scrape_success` | 所有启用采集器都成功才为 1 |
| `ikuai_collector_success{collector}` | system / interfaces / clients_ipv4 / clients_ipv6 成功状态 |
| `ikuai_collector_duration_seconds{collector}` | 各采集器耗时，包含分页 |
| `ikuai_uptime_seconds{id}` | 主机或可查询线路的运行秒数 |
| `ikuai_cpu_usage_ratio{id}` | 0–1，all 为平均，core/0 起为单核 |
| `ikuai_cpu_temperature_celsius{sensor}` | 摄氏度 |
| `ikuai_memory_{total,used,available,cached,buffers}_bytes` | 字节；从路由器 KB 字段乘 1024 |
| `ikuai_interface_info` / `ikuai_device_info` | 接口 / 终端身份与备注 |
| `ikuai_device_count` | 系统接口报告的在线终端数 |
| `ikuai_network_{upload,download}_total_bytes` | Counter，重启归零后由 Prometheus rate 正确处理 |
| `ikuai_network_{upload,download}_bytes_per_second` | 路由器即时速率，字节/秒 |
| `ikuai_network_connections` | 当前连接数；`--` 不生成虚假零值 |

一次 scrape 并行读取各监控接口，完整翻页，并按接口名、终端 IP 去重。某个接口失败时保留其他成功数据，报告对应失败状态；不会返回缓存伪装成功。并发 scrape 共享正在进行的读取。

## Go 模块与开发

```sh
go get github.com/NERVEbing/ikuai-aio/v4@v4
make test
make vet
make build
```

```go
client, err := api.NewClient(api.Options{
    Address: "http://192.168.1.1",
    Token: os.Getenv("IKUAI_TOKEN"),
    Timeout: 10 * time.Second,
    ReadRetries: 2,
})
if err != nil { return err }
defer client.Close()
info, err := client.System(ctx)
```

使用 Go 1.26+。命令 `ikuai-aio check-config` 只校验配置，`ikuai-aio healthcheck` 检查本服务。SIGINT / SIGTERM 会取消采集、下载和排队任务，并关闭 HTTP 服务。日志为 JSON，库代码不调用 `log.Fatal`。

CI 在 `v4` 推送和 PR 时执行格式、vet、race 测试与 amd64 / arm64 编译。发布 `v4.*` 标签触发镜像发布，标签为 `ghcr.io/nervebing/ikuai-aio:v4`，不会覆盖旧分支的 `latest`。

[接口依据与验证范围](docs/api-v4.md) · [从旧版迁移](docs/migration-v4.md) · [MIT License](LICENSE)
