# HeroMail

HeroMail 是一个可自托管的邮箱资源池与注册验证码平台。平台管理员维护 Outlook/Hotmail 邮箱资产和目标平台的收码规则；平台用户选择要注册的目标平台，由系统自动分配一个邮箱，用户把邮箱用于平台注册后，在 HeroMail 中获取验证码。

它解决的是“多个注册平台、多个邮箱资产、统一收码和统一调度”的问题。使用者不需要把邮箱密码、OAuth Token 或完整收件箱暴露给最终用户，管理员也可以按“邮箱 × 目标平台”追踪占用，避免同一个邮箱在同一平台重复分配。

## 主要收益

- **统一入口**：用户只选择目标平台，不需要理解邮箱供应商和收信协议。
- **资产可控**：管理员可以看到邮箱健康、授权状态、池归属和平台占用。
- **业务不串号**：平台规则限制发件人、主题和验证码解析范围。
- **自动结算**：分配时预扣额度，收码成功结算，超时未收到自动退款。
- **可自托管**：代码、数据和凭证留在自己的服务器，适合内部平台或受控的开发测试环境。
- **可扩展**：邮箱接入、目标平台规则和订单状态彼此解耦，后续可以增加 Graph、IMAP 或其他渠道。

## 当前状态

仓库当前提供可运行的 MVP：

- Go + Gin HTTP 服务。
- 用户端：选择目标平台、申请系统邮箱、提交注册、获取模拟验证码、完成/取消订单、查看订单记录。
- 管理员端：运行概览、邮箱资源、目标平台和注册订单查询。
- 领域层包含邮箱状态、邮箱与平台占用状态、订单状态机、余额预扣和超时退款。
- 默认使用内存存储，便于快速体验；存储接口和 Docker 配置为接入 PostgreSQL/Redis 预留位置。

产品和技术设计见 [`docs/final-product-design.md`](docs/final-product-design.md)。

## 快速开始

需要 Go 1.26 或更高版本：

```bash
go test ./...
go run ./cmd/heromail
```

浏览器打开 <http://localhost:8080>。页面右上角可以切换用户端和管理员端。

使用 Docker Compose 体验完整的本地依赖拓扑：

```bash
cp .env.example .env
docker compose up --build
```

当前 MVP 仍使用内存存储，Compose 中的 PostgreSQL 和 Redis 是后续持久化、锁、队列和限流的开发依赖预留，不会自动写入业务数据。

演示身份通过请求头区分：

```text
X-HeroMail-Role: user
X-HeroMail-User: user-001
X-HeroMail-Role: admin
X-HeroMail-User: admin-001
```

这只是演示模式，不能直接用于公网生产环境。生产部署必须接入真实登录、会话、权限和密钥管理。

## 业务流程

```text
选择目标平台
  -> 预扣额度并分配 Outlook/Hotmail 邮箱
  -> 用户把邮箱填入目标平台注册
  -> 点击“我已提交注册”
  -> 邮件 Worker 按平台规则匹配并提取验证码
  -> 用户完成注册，或任务超时/取消
  -> 结算订单并更新邮箱的平台占用
```

同一个邮箱可以用于不同目标平台；同一个邮箱在某个平台收到验证码后，默认标记为该平台已消费，不再重复分配。用户不能查看邮箱密码、OAuth Token 或完整邮件正文。

## API 示例

查看目标平台：`curl http://localhost:8080/api/v1/services`

申请邮箱：

```bash
curl -X POST http://localhost:8080/api/v1/orders -H 'Content-Type: application/json' -H 'X-HeroMail-User: user-001' -d '{"service":"github","request_id":"demo-001"}'
```

提交注册并等待验证码：

```bash
curl -X POST http://localhost:8080/api/v1/orders/ORD001001/submitted -H 'X-HeroMail-User: user-001'
```

MVP 会在提交后生成演示验证码。真实接入时由 Outlook Graph/IMAP Worker 写入相同的订单状态机。

## 技术架构

```text
Vue/原生前端 -> Go/Gin API -> 订单与调度领域层
                             |
                 PostgreSQL 最终数据源
                 Redis 锁、租约、队列和限流
                             |
                 Graph/IMAP 邮件适配器
```

当前前端使用原生 HTML/CSS/JavaScript 并由 Go 嵌入，减少开源项目初始化成本。后续可以替换为 Vue 3/Vite，不改变 API 和领域层。

## 安全边界

- 邮箱凭证必须加密存储，不能写入日志、截图或 Git。
- OAuth 仅申请读取邮件和离线刷新权限，不向用户暴露 Token。
- 只匹配目标平台允许的发件人、域名、主题和验证码格式。
- 验证码和邮件元数据应设置短保留期，默认不保存完整邮件正文。
- 公网部署前必须增加真实认证、API Key 哈希存储、限流、IP 白名单、审计日志和滥用处置。
- 本项目只应用于已授权的邮箱和合规的注册测试，不得用于撞库、绕过平台风控或批量滥用。

## 目录结构

```text
cmd/heromail/          服务入口
internal/domain/       领域模型和状态定义
internal/store/        存储接口的内存实现和单元测试
internal/http/         Gin API 和权限边界
internal/web/static/   嵌入式用户端/管理员端页面
docs/                  产品与技术设计
AGENTS.md              Agent 协作和修改约束
```

## 开发与提交

提交前执行：

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```

提交信息、代码注释和项目文档统一使用中文。每个提交只完成一个可验证的阶段，避免把大批生成文件和业务代码混在一起。

## 许可证

本项目采用 MIT 许可证，见 [`LICENSE`](LICENSE)。
