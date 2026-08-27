# HeroMail

HeroMail 是一个可自托管的邮箱资源池与平台注册验证码管理系统。管理员集中维护多种邮箱、平台规则和按邮箱类型设置的价格；用户选择目标平台及一个或多个可接受的邮箱类型后，系统便会分配可用邮箱并立即开始获取验证码。

与直接共享邮箱凭证相比，HeroMail 只向用户暴露本次任务的邮箱地址和验证码。邮箱密码、OAuth Token 和完整收件箱由平台加密保管；“邮箱 × 目标平台”状态又能防止同一邮箱在同一平台重复分配。

网页按三层组织：未登录即可访问的公开官网（产品介绍、价格、API 文档、服务状态和开源部署）、登录后的用户门户（申请邮箱、当前任务、订单、余额和开发者能力）以及仅管理员可进入的运营后台（资源、规则、订单、支付和审计）。公开页面不泄露邮箱库存、内部收码规则或任何凭证；真正创建订单必须先登录。

常用入口：`/` 公开首页、`/pricing` 价格、`/docs` 文档、`/login` 登录、`/app` 用户门户、`/admin` 管理后台。用户门户采用顶部导航，移动端采用底部导航；管理员后台继续使用侧栏，以便处理高密度运营任务。

## 为什么使用 HeroMail

- 用户只选择要注册的平台，无需了解邮箱渠道和收信协议。
- 管理员可查看邮箱池、授权、健康度、平台占用和注册订单。
- 创建订单时原子扣除余额并立即收码，30 分钟超时未收码自动退款。
- 支持 Microsoft Graph OAuth2、邮件去重、发件人/主题/正则匹配和 Webhook 通知。
- 支持易支付和支付宝官方通道，回调验签通过后幂等入账。
- PostgreSQL 作为最终数据源，Redis 承担跨进程锁、OAuth 状态和协调。
- 提供 Docker 一键部署、GitHub Actions 多架构镜像和管理端在线升级。

## 一键部署

在拥有 root 或 sudo 权限的 Linux 服务器执行：

```bash
curl -fsSL https://github.com/ljunn/heromail/releases/latest/download/install.sh | sudo bash
```

脚本会检查 Git、Docker 和 Docker Compose，自动定位 GitHub 最新正式 Release，将对应标签安装到 `/opt/heromail`，生成数据库密码、管理员密码、加密主密钥和 Worker 令牌，然后启动 HeroMail、PostgreSQL、Redis 和升级执行器。已安装的环境不允许通过重跑脚本升级，必须由 GitHub Release 工作流通过 SSH 调用升级脚本。

自定义安装目录、端口和公网地址：

```bash
curl -fsSL https://github.com/ljunn/heromail/releases/latest/download/install.sh | \
  sudo HEROMAIL_DIR=/srv/heromail HEROMAIL_PORT=9090 \
  HEROMAIL_PUBLIC_URL=https://mail.example.com bash
```

部署完成后，使用 `/opt/heromail/.env` 中的 `HEROMAIL_ADMIN_EMAIL` 和 `HEROMAIL_ADMIN_PASSWORD` 登录。管理员可在“管理员账户”中修改登录密码，修改后旧会话会全部失效。请在反向代理中配置 HTTPS，并确保 `HEROMAIL_PUBLIC_URL` 是外部可访问的绝对地址，否则支付回调和 OAuth 无法正常工作。

## 业务流程

```text
用户选择目标平台和可接受的邮箱类型
  -> 系统在事务中扣除余额、分配邮箱并开始收码
  -> 用户把邮箱填入目标平台
  -> Graph Worker 拉取邮件并按平台规则匹配
  -> 订单写入验证码并通知用户/Webhook
  -> 后台归档订单，或 30 分钟超时后自动退款
```

同一邮箱可以服务不同目标平台；同一邮箱在某个平台收到验证码后，该组合默认标记为已消费，不会再次分配。同一组合累计 5 次超时未收码，也会标记为已消费并停止分配。

## 管理端配置

### Microsoft Graph

1. 在 Microsoft Entra 中创建 Web 应用，并配置委托权限 `User.Read`、`Mail.Read` 和 `offline_access`。
2. 将回调地址设为 `https://你的域名/api/v1/oauth/microsoft/callback`。
3. 在 `.env` 填写 `MICROSOFT_CLIENT_ID`、`MICROSOFT_CLIENT_SECRET`、`MICROSOFT_TENANT` 和 `MICROSOFT_REDIRECT_URI`。

### Google Gmail OAuth

管理员在“接入渠道”点击“连接 Google 邮箱”后，会跳转到 Google 官方登录页完成登录和 2FA，授权成功后 HeroMail 使用 Gmail IMAP XOAUTH2 读取邮件。Google 密码和 `2fa.live` 地址不会被用于模拟网页登录。

1. 在 Google Cloud 创建“Web 应用” OAuth 客户端。
2. 将 `https://heromail.cc/api/v1/oauth/google/callback` 添加为已获授权的重定向 URI；本地开发可另加对应的 `http://localhost` 地址。
3. 登录管理员后台，在“接入渠道”的 Google Gmail OAuth2 卡片填写并保存 Google OAuth 三项配置；也兼容在服务环境变量中预配置。
4. OAuth 同意屏幕至少申请 `openid`、`email`、`profile` 和 `https://mail.google.com/` 权限。公开应用使用 Gmail 全量邮件权限时，按 Google 要求完成测试用户和应用审核。
5. 在“接入渠道”中点击“连接 Google 邮箱”。授权成功后邮箱会自动进入验证队列。

Google OAuth 同意屏幕需要填写公开的应用隐私权政策和服务条款链接：

- 隐私政策：https://heromail.cc/privacy
- 服务条款：https://heromail.cc/terms

### 易支付

在“支付管理”新建易支付服务商，填写提交 API 地址、PID、商户密钥和可选的支付宝通道 ID。HeroMail 使用 MD5 生成支付参数签名，并对异步回调的 PID、签名、交易状态和金额同时校验。

### 支付宝官方

在“支付管理”新建支付宝官方服务商，填写 AppID、应用 RSA 私钥和支付宝 RSA 公钥。官方网关固定为 `https://openapi.alipay.com/gateway.do`，无需手动填写。PC 端使用 `alipay.trade.page.pay`，移动端使用 `alipay.trade.wap.pay`，签名算法为 RSA2。私钥和支付服务商配置均使用 AES-256-GCM 加密存储；编辑时敏感字段留空会保留原值。

## 在线升级

一键部署会启动独立升级执行器。只有创建 `v*` GitHub Release 时，工作流才会更新 `ghcr.io/ljunn/heromail:latest`，随后自动通过 SSH 调用服务器上的 `scripts/upgrade.sh <版本标签>`。升级脚本会先创建并校验 PostgreSQL 压缩备份，再拉取带版本号的官方 GHCR 镜像并切换 HeroMail 容器；健康检查或版本标签校验失败会自动恢复上一镜像，PostgreSQL 和 Redis 数据卷不会被删除。升级请求、失败和成功都会写入审计日志。

自动升级需要在 GitHub 仓库 Secrets 中配置 `SERVER_HOST`、`SERVER_USER`，并配置 `SERVER_SSH_KEY` 或 `SERVER_PASSWORD` 其中一种认证方式；可选 `SERVER_PORT`（默认 `22`）和 `SERVER_APP_HOME`（默认 `/opt/heromail`）。SSH 用户必须是 root，或具备无需交互输入密码的 `sudo` 权限。

每个正式版本都必须先在 [`CHANGELOG.md`](CHANGELOG.md) 增加同版本章节。发布工作流会使用该章节作为 GitHub Release 正文，缺少对应更新日志时会直接阻止发布。

系统会在在线升级前自动备份数据库；也可以在维护窗口手动执行备份：

```bash
cd /opt/heromail
docker compose exec -T postgres pg_dump -U heromail heromail | gzip > heromail-backup.sql.gz
```

## 本地开发

需要 Go 1.26 或更高版本。单元测试使用显式内存存储，不依赖外部服务：

```bash
go test ./...
go vet ./...
HEROMAIL_STORAGE=memory go run ./cmd/heromail
```

要验证真实持久化链路，使用 Docker Compose：

```bash
cp .env.example .env
# 将 .env 中的“请替换”值换成强随机密钥
docker compose up --build
```

浏览器打开 <http://localhost:8080>。生产模式不使用伪造身份请求头，页面和 API 都使用 Bearer 会话或 `hm_` 前缀 API Key。

## API 示例

登录并保存返回的会话令牌：

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"你的登录密码"}'
```

使用令牌创建注册订单：

```bash
curl -X POST http://localhost:8080/api/v1/orders \
  -H 'Authorization: Bearer 会话令牌' \
  -H 'Content-Type: application/json' \
  -d '{"service":"github","request_id":"request-001"}'
```

所有列表接口支持 `page` 和 `page_size`，并返回统一分页信息：

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 0,
    "total_pages": 0
  }
}
```

## 架构

```text
浏览器 / API Key
        |
   Go + Gin API
        |
        +-- PostgreSQL：用户、邮箱、订单、余额、支付、Webhook、审计
        +-- Redis：分配锁、OAuth 一次性状态、跨进程协调
        +-- Microsoft Graph Worker：Token 刷新、邮件拉取、规则匹配
        +-- Webhook Worker：HMAC 签名、指数退避、手动重试
        +-- 支付适配器：易支付 MD5 / 支付宝官方 RSA2
```

| 目录 | 用途 |
| --- | --- |
| `cmd/heromail/` | 服务入口与 Worker 启动 |
| `internal/domain/` | 订单、邮箱、支付和 Webhook 领域模型 |
| `internal/store/` | PostgreSQL/Redis 存储、事务和内存测试实现 |
| `internal/mail/` | Microsoft Graph OAuth 与收码 Worker |
| `internal/payment/` | 易支付与支付宝官方适配器 |
| `internal/webhook/` | Webhook 投递 Worker |
| `internal/http/` | Gin API、认证与权限边界 |
| `internal/web/static/` | 无构建依赖的用户端与管理端 |
| `scripts/install.sh` | Linux 正式版本一键安装 |

## 安全与合规

- 不要把 `.env`、邮箱凭证、支付私钥或真实邮箱数据提交到 Git。
- 生产环境使用至少 32 字节随机主密钥，仅通过 HTTPS 暴露服务。
- Webhook 默认拒绝回环、私网和链路本地地址；仅本地受控测试才应开启私网投递。
- 本项目只用于已授权邮箱和合规的注册测试，不得用于撞库、绕过平台风控或批量滥用。

## 开发与贡献

提交前执行：

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```

项目代码注释、提交信息和项目文档使用中文。GitHub Actions 会在每次提交后运行格式、测试和静态检查，生成 Linux amd64/arm64 安装包，并在 push 时发布多架构 GHCR 镜像。

## 许可证

本项目采用 MIT 许可证，见 [`LICENSE`](LICENSE)。
