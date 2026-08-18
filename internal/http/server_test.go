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
	if !strings.Contains(indexResponse.Body.String(), "/app.js?v=static-test-commit") || !strings.Contains(indexResponse.Body.String(), "/styles.css?v=static-test-commit") {
		t.Fatal("入口页面没有使用构建版本隔离静态资源缓存")
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
