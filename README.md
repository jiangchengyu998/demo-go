# Cloud Deploy Go Demo

云朵一键部署平台 Go demo。项目参考
[demo-python](../demo-python)，保留同一个业务资源 `items` 的 CRUD，并兼容 Java/Python demo 使用的
MySQL 数据库 `cloud_deploy_demo` 和表结构。

## 技术栈

- Go 1.22+
- 标准库 `net/http`
- `database/sql` / MySQL
- OpenAPI / Swagger UI
- Prometheus metrics
- OpenTelemetry OTLP tracing

## 本地启动

```bash
go mod tidy
go run ./cmd/demo-go
```

不设置数据库环境变量时，应用会使用进程内内存存储，便于直接启动和调试。容器环境默认监听 `8000`，
并优先读取平台注入的 `PORT`，同时兼容 `SERVER_PORT`。

## 使用同一个 MySQL 数据库

应用兼容 Java demo 的环境变量，适合一键部署平台直接复用同一个数据库和同一张表：

```text
database: cloud_deploy_demo
table: items
username: root
password: 通过 SPRING_DATASOURCE_PASSWORD 环境变量注入
```

```bash
SPRING_DATASOURCE_URL='jdbc:mysql://host:3306/cloud_deploy_demo?createDatabaseIfNotExist=true&useUnicode=true&characterEncoding=utf8&useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai' \
SPRING_DATASOURCE_USERNAME=root \
SPRING_DATASOURCE_PASSWORD=secret \
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://opentelemetry-collector.observability.svc.cluster.local:4318/v1/traces \
go run ./cmd/demo-go
```

也支持 Go 常用的 MySQL URL 和 Python demo 常见的 `mysql+pymysql://` URL，优先级高于
`SPRING_DATASOURCE_URL`：

```bash
DATABASE_URL='mysql://root:secret@host:3306/cloud_deploy_demo?charset=utf8mb4'
```

## 接口

- `GET /api/items`：分页查询，支持 `page`、`size`、`sort`
- `GET /api/items/{id}`：按 ID 查询
- `POST /api/items`：创建
- `PUT /api/items/{id}`：更新
- `DELETE /api/items/{id}`：删除
- `GET /swagger-ui.html`：Swagger UI
- `GET /v3/api-docs`：OpenAPI JSON
- `GET /actuator/health`：健康检查
- `GET /actuator/prometheus`：Prometheus 指标

创建示例：

```bash
curl -X POST http://localhost:8000/api/items \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo item","description":"created from curl"}'
```

## 数据库

数据库名：`cloud_deploy_demo`

应用启动时会自动创建数据库，并执行同等迁移。表结构与 Java/Python demo 保持一致：

```sql
CREATE TABLE IF NOT EXISTS items (
    id BIGINT NOT NULL AUTO_INCREMENT,
    name VARCHAR(80) NOT NULL,
    description VARCHAR(500) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

## OTEL

默认 trace endpoint：

```text
http://opentelemetry-collector.observability.svc.cluster.local:4318/v1/traces
```

本地默认不设置 `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` 时会关闭 OTEL exporter，避免因为无法解析集群内
`opentelemetry-collector.observability.svc.cluster.local` 导致本地日志刷连接错误。

部署到平台时建议注入：

```text
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://opentelemetry-collector.observability.svc.cluster.local:4318/v1/traces
DEPLOYMENT_ENVIRONMENT=dev
```

## 验证

```bash
go test ./...
go build ./cmd/demo-go
```

## 一键部署平台构建

当前仓库不提交 Dockerfile，让平台使用 Cloud Native Buildpacks 构建镜像。`Procfile` 指定默认 Web
进程为 `demo-go`，启动时会读取 `PORT` 或 `SERVER_PORT`，默认监听 `8000`。

Kubernetes/Helm 侧建议配置：

```yaml
container:
  port: 8000
service:
  port: 80
```

应用镜像由非 root 用户运行，监听 `8000` 比监听 `80` 更符合容器最佳实践；Service 继续对外暴露
`80`，再转发到 Pod 的 `8000`。
