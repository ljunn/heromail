package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ljunn/heromail/internal/buildinfo"
	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

func TestCreateOrderAndReceiveCode(t *testing.T) {
	server := NewServer(store.New())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(`{"service":"github","request_id":"http-test-001"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-HeroMail-User", "user-001")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("创建订单返回 %d，响应：%s", response.Code, response.Body.String())
	}
	var created struct {
		Data domain.Order `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析创建订单响应失败：%v", err)
	}
	if created.Data.Status != domain.OrderAssigned || created.Data.MailboxAddress == "" {
		t.Fatalf("订单分配结果不正确：%+v", created.Data)
	}

	submit := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+created.Data.ID+"/submitted", nil)
	submit.Header.Set("X-HeroMail-User", "user-001")
	submitResponse := httptest.NewRecorder()
	server.Router.ServeHTTP(submitResponse, submit)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("提交订单返回 %d，响应：%s", submitResponse.Code, submitResponse.Body.String())
	}

	time.Sleep(1700 * time.Millisecond)
	order, ok := server.Store.GetOrder(created.Data.ID)
	if !ok || order.Status != domain.OrderCodeReceived || order.Code == "" {
		t.Fatalf("演示验证码没有写入订单：%+v", order)
	}
}

func TestAdminEndpointRequiresRole(t *testing.T) {
	server := NewServer(store.New())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("无管理员身份时返回 %d，期望 %d", response.Code, http.StatusForbidden)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	request.Header.Set("X-HeroMail-User", "admin-001")
	response = httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("管理员访问返回 %d，响应：%s", response.Code, response.Body.String())
	}
}

func TestStaticAssetsUseBuildVersion(t *testing.T) {
	originalCommit := buildinfo.Commit
	buildinfo.Commit = "static-test-commit"
	t.Cleanup(func() { buildinfo.Commit = originalCommit })
	server := NewServer(store.New())

	indexRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	indexResponse := httptest.NewRecorder()
	server.Router.ServeHTTP(indexResponse, indexRequest)
	if indexResponse.Code != http.StatusOK {
		t.Fatalf("入口页面返回 %d，期望 %d", indexResponse.Code, http.StatusOK)
	}
	if indexResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("入口页面缓存策略不正确：%s", indexResponse.Header().Get("Cache-Control"))
	}
	if !strings.Contains(indexResponse.Body.String(), "/public.js?v=static-test-commit") || !strings.Contains(indexResponse.Body.String(), "/public.css?v=static-test-commit") {
		t.Fatal("公开站没有使用构建版本隔离静态资源缓存")
	}
	if !strings.Contains(indexResponse.Body.String(), "gsap@3.15.0/dist/gsap.min.js") || !strings.Contains(indexResponse.Body.String(), "gsap@3.15.0/dist/ScrollTrigger.min.js") {
		t.Fatal("公开站没有加载固定版本的 GSAP 和 ScrollTrigger")
	}
	workspaceRequest := httptest.NewRequest(http.MethodGet, "/app/tasks", nil)
	workspaceResponse := httptest.NewRecorder()
	server.Router.ServeHTTP(workspaceResponse, workspaceRequest)
	if workspaceResponse.Code != http.StatusOK {
		t.Fatalf("工作台深层路由返回 %d，期望 %d", workspaceResponse.Code, http.StatusOK)
	}
	if workspaceResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("工作台入口缓存策略不正确：%s", workspaceResponse.Header().Get("Cache-Control"))
	}
	if !strings.Contains(workspaceResponse.Body.String(), "/app.js?v=static-test-commit") || !strings.Contains(workspaceResponse.Body.String(), "/styles.css?v=static-test-commit") {
		t.Fatal("工作台没有使用构建版本隔离静态资源缓存")
	}

	assetRequest := httptest.NewRequest(http.MethodGet, "/app.js?v=static-test-commit", nil)
	assetResponse := httptest.NewRecorder()
	server.Router.ServeHTTP(assetResponse, assetRequest)
	if assetResponse.Code != http.StatusOK {
		t.Fatalf("静态资源返回 %d，期望 %d", assetResponse.Code, http.StatusOK)
	}
	if assetResponse.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("静态资源缓存策略不正确：%s", assetResponse.Header().Get("Cache-Control"))
	}
}

func TestRemovedStatusPageRedirectsHome(t *testing.T) {
	server := NewServer(store.New())
	request := httptest.NewRequest(http.MethodGet, "/status", nil)
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("旧服务状态页面返回 %d，期望 %d", response.Code, http.StatusFound)
	}
	if location := response.Header().Get("Location"); location != "/" {
		t.Fatalf("旧服务状态页面跳转到 %q，期望首页", location)
	}
}

func TestPublicServicesArePaginatedAndHideInternalRules(t *testing.T) {
	server := NewServer(store.New())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/services?page=1&page_size=2", nil)
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("公开平台列表返回 %d，响应：%s", response.Code, response.Body.String())
	}

	var body struct {
		Data       []map[string]any `json:"data"`
		Pagination struct {
			Page       int   `json:"page"`
			PageSize   int   `json:"page_size"`
			Total      int64 `json:"total"`
			TotalPages int   `json:"total_pages"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析公开平台列表失败：%v", err)
	}
	if len(body.Data) > 2 || body.Pagination.Page != 1 || body.Pagination.PageSize != 2 || body.Pagination.Total < int64(len(body.Data)) {
		t.Fatalf("公开平台分页不正确：%+v", body.Pagination)
	}
	for _, service := range body.Data {
		for _, field := range []string{"available_mailboxes", "leased_mailboxes", "consumed_mailboxes", "sender_domains", "subject_keywords", "regex", "enabled"} {
			if _, exists := service[field]; exists {
				t.Fatalf("公开平台响应泄露内部字段 %s：%+v", field, service)
			}
		}
	}
}

func TestPublicAndAdminPagesUseSeparateEntries(t *testing.T) {
	server := NewServer(store.New())
	for _, path := range []string{"/pricing", "/docs/api", "/open-source", "/login", "/register"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		server.Router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "public-content") {
			t.Fatalf("公开路由 %s 没有返回公开站入口：%d", path, response.Code)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "user-nav") || strings.Contains(response.Body.String(), "public-content") {
		t.Fatalf("管理后台深层路由没有返回独立工作台入口：%d", response.Code)
	}
}

func TestOnlineUpgradeCreatesRequestForAdmin(t *testing.T) {
	directory := t.TempDir()
	requestPath := directory + "/request.json"
	statusPath := directory + "/status.json"
	t.Setenv("HEROMAIL_UPGRADE_REQUEST", requestPath)
	t.Setenv("HEROMAIL_UPGRADE_STATUS", statusPath)
	server := NewServer(store.New())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/upgrade", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("创建升级任务返回 %d，响应：%s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(requestPath); err != nil {
		t.Fatalf("升级请求文件不存在：%v", err)
	}
	status, err := readUpgradeStatus(statusPath)
	if err != nil || status.State != "queued" {
		t.Fatalf("升级状态不正确：%+v，错误：%v", status, err)
	}
}

func TestOnlineUpgradeRejectsCurrentRelease(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HEROMAIL_UPGRADE_REQUEST", directory+"/request.json")
	t.Setenv("HEROMAIL_UPGRADE_STATUS", directory+"/status.json")
	originalVersion := buildinfo.Version
	buildinfo.Version = "v1.0.3"
	t.Cleanup(func() { buildinfo.Version = originalVersion })
	server := NewServer(store.New())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/upgrade", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	request.Header.Set("X-HeroMail-Target-Version", "v1.0.3")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("同版本升级返回 %d，期望 %d，响应：%s", response.Code, http.StatusConflict, response.Body.String())
	}
}

type resourceRepositoryStub struct {
	store.Repository
	store.ResourceRepository
	savedService domain.Service
}

func (s *resourceRepositoryStub) SaveService(_ string, service domain.Service, _ string) (domain.Service, error) {
	s.savedService = service
	return service, nil
}

func TestAdminSaveServiceValidatesConfigPrecisely(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "缺少发件人域名",
			body:        `{"code":"Grok","name":"grok 注册","enabled":true,"allowed_providers":["outlook","hotmail"],"price":0.02,"ttl_seconds":600,"sender_domains":[],"subject_keywords":[],"regex":"\\b(\\d{6})\\b"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "至少填写一个发件人域名",
		},
		{
			name:        "不支持的邮箱供应商",
			body:        `{"code":"grok","name":"Grok 注册","enabled":true,"allowed_providers":["gmail"],"price":0.02,"ttl_seconds":600,"sender_domains":["x.ai"],"regex":"\\b(\\d{6})\\b"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "只允许 outlook 和 hotmail",
		},
		{
			name:       "有效配置",
			body:       `{"code":"Grok","name":"grok 注册","enabled":true,"allowed_providers":["outlook","hotmail"],"price":0.02,"ttl_seconds":600,"sender_domains":["x.ai"],"subject_keywords":[],"regex":"\\b(\\d{6})\\b"}`,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &resourceRepositoryStub{Repository: store.New()}
			server := NewServer(repository)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/services", bytes.NewBufferString(tt.body))
			request.Header.Set("X-HeroMail-Role", "admin")
			request.Header.Set("X-HeroMail-User", "admin-001")
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			server.Router.ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("保存目标平台返回 %d，期望 %d，响应：%s", response.Code, tt.wantStatus, response.Body.String())
			}
			if tt.wantMessage != "" && !strings.Contains(response.Body.String(), tt.wantMessage) {
				t.Fatalf("错误提示不准确：%s", response.Body.String())
			}
			if tt.wantStatus == http.StatusOK && repository.savedService.Code != "grok" {
				t.Fatalf("平台代码没有标准化：%q", repository.savedService.Code)
			}
		})
	}
}

type accountRepositoryStub struct {
	store.Repository
	store.AccountRepository
	user            domain.User
	changedUserID   string
	currentPassword string
	newPassword     string
	auditAction     string
}

func (s *accountRepositoryStub) ResolveAccessToken(token string) (domain.User, bool) {
	return s.user, token == "有效会话"
}

func (s *accountRepositoryStub) ChangePassword(userID, currentPassword, newPassword string) (string, error) {
	s.changedUserID = userID
	s.currentPassword = currentPassword
	s.newPassword = newPassword
	return "新会话", nil
}

func (s *accountRepositoryStub) WriteAudit(_ string, action, _, _, _, _ string) error {
	s.auditAction = action
	return nil
}

func TestAdminCanChangePassword(t *testing.T) {
	repository := &accountRepositoryStub{
		Repository: store.New(),
		user:       domain.User{ID: "admin-001", Email: "admin@example.com", Role: "admin", Status: "active"},
	}
	server := NewServer(repository)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/me/password", bytes.NewBufferString(`{"current_password":"旧管理员密码","new_password":"AdminNewPassword123"}`))
	request.Header.Set("Authorization", "Bearer 有效会话")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("修改管理员密码返回 %d，响应：%s", response.Code, response.Body.String())
	}
	if repository.changedUserID != "admin-001" || repository.currentPassword != "旧管理员密码" || repository.newPassword != "AdminNewPassword123" {
		t.Fatalf("密码修改参数不正确：%+v", repository)
	}
	if repository.auditAction != "admin.password.change" {
		t.Fatalf("管理员改密审计动作不正确：%s", repository.auditAction)
	}
	if !strings.Contains(response.Body.String(), "新会话") {
		t.Fatalf("响应没有返回新会话：%s", response.Body.String())
	}
}

type paymentRepositoryStub struct {
	store.Repository
	store.PaymentRepository
	secret      store.PaymentProviderSecret
	saved       domain.PaymentProvider
	savedConfig map[string]string
	deleteErr   error
}

func (s *paymentRepositoryStub) GetPaymentProviderSecret(string) (store.PaymentProviderSecret, error) {
	return s.secret, nil
}

func (s *paymentRepositoryStub) SavePaymentProvider(_ string, provider domain.PaymentProvider, config map[string]string, _ string) (domain.PaymentProvider, error) {
	s.saved = provider
	s.savedConfig = config
	return provider, nil
}

func (s *paymentRepositoryStub) DeletePaymentProvider(_, _, _ string) error {
	return s.deleteErr
}

func TestOfficialAlipayUsesPresetGatewayAndKeepsSecrets(t *testing.T) {
	repository := &paymentRepositoryStub{
		Repository: store.New(),
		secret: store.PaymentProviderSecret{
			Provider: domain.PaymentProvider{ID: "provider-001", Type: "alipay"},
			Config: map[string]string{
				"private_key": "已有应用私钥",
				"public_key":  "已有支付宝公钥",
			},
		},
	}
	server := NewServer(repository)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/payment/providers", bytes.NewBufferString(`{"id":"provider-001","name":"支付宝官方","type":"alipay","methods":["alipay"],"enabled":true,"priority":10,"config":{"gateway":"https://example.com/错误网关","app_id":"2026000000000000","private_key":"","public_key":""}}`))
	request.Header.Set("X-HeroMail-Role", "admin")
	request.Header.Set("X-HeroMail-User", "admin-001")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("保存支付宝官方配置返回 %d，响应：%s", response.Code, response.Body.String())
	}
	if repository.savedConfig["gateway"] != "https://openapi.alipay.com/gateway.do" {
		t.Fatalf("支付宝网关没有固定为官方地址：%s", repository.savedConfig["gateway"])
	}
	if repository.savedConfig["private_key"] != "已有应用私钥" || repository.savedConfig["public_key"] != "已有支付宝公钥" {
		t.Fatalf("编辑时没有保留已有 RSA 密钥：%+v", repository.savedConfig)
	}
}

func TestEasyPayRequiresIndependentFields(t *testing.T) {
	repository := &paymentRepositoryStub{Repository: store.New()}
	server := NewServer(repository)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/payment/providers", bytes.NewBufferString(`{"name":"易支付","type":"easypay","methods":["alipay"],"enabled":true,"priority":10,"config":{"api_base":"https://pay.example.com/submit.php","pid":"10001"}}`))
	request.Header.Set("X-HeroMail-Role", "admin")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "商户密钥") {
		t.Fatalf("易支付缺少 PKey 返回 %d，响应：%s", response.Code, response.Body.String())
	}
}

func TestPaymentProviderWithPendingOrdersCannotBeDeleted(t *testing.T) {
	repository := &paymentRepositoryStub{Repository: store.New(), deleteErr: store.ErrPaymentProviderInUse}
	server := NewServer(repository)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/payment/providers/provider-001", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)

	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "待支付订单") {
		t.Fatalf("删除使用中的服务商返回 %d，响应：%s", response.Code, response.Body.String())
	}
}
