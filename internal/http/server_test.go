package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"mime/multipart"
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

func TestCreateOrderAutomaticallyListensWithoutUserMutationEndpoints(t *testing.T) {
	server := NewServer(store.New())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(`{"service":"adobe","request_id":"http-test-001"}`))
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
	if created.Data.Status != domain.OrderWaitingCode || created.Data.SubmittedAt.IsZero() || created.Data.MailboxAddress == "" {
		t.Fatalf("订单分配结果不正确：%+v", created.Data)
	}
	if created.Data.ExpiresAt.Sub(created.Data.CreatedAt) < 30*time.Minute {
		t.Fatalf("订单有效期 = %s，期望至少 30 分钟", created.Data.ExpiresAt.Sub(created.Data.CreatedAt))
	}
	for _, action := range []string{"submitted", "complete", "cancel"} {
		mutation := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+created.Data.ID+"/"+action, nil)
		mutation.Header.Set("X-HeroMail-User", "user-001")
		mutationResponse := httptest.NewRecorder()
		server.Router.ServeHTTP(mutationResponse, mutation)
		if mutationResponse.Code != http.StatusNotFound {
			t.Fatalf("用户状态接口 %s 返回 %d，期望 %d", action, mutationResponse.Code, http.StatusNotFound)
		}
	}

	time.Sleep(1700 * time.Millisecond)
	order, ok := server.Store.GetOrder(created.Data.ID)
	if !ok || order.Status != domain.OrderWaitingCode || order.Code != "" {
		t.Fatalf("没有真实匹配邮件时订单不应生成验证码：%+v", order)
	}
}

func TestAdminOwnsOrderCompletionAndCancellation(t *testing.T) {
	repository := store.New()
	server := NewServer(repository)
	waiting, err := repository.CreateOrder("user-001", "svc-adobe", "admin-cancel")
	if err != nil {
		t.Fatalf("创建待收码订单失败：%v", err)
	}
	cancel := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orders/"+waiting.ID+"/cancel", nil)
	cancel.Header.Set("X-HeroMail-User", "admin-001")
	cancel.Header.Set("X-HeroMail-Role", "admin")
	cancelResponse := httptest.NewRecorder()
	server.Router.ServeHTTP(cancelResponse, cancel)
	if cancelResponse.Code != http.StatusOK {
		t.Fatalf("管理员取消返回 %d：%s", cancelResponse.Code, cancelResponse.Body.String())
	}

	received, err := repository.CreateOrder("user-001", "svc-openai", "admin-complete")
	if err != nil {
		t.Fatalf("创建收码订单失败：%v", err)
	}
	if _, err := repository.ReceiveCodeValue(received.ID, "314159"); err != nil {
		t.Fatalf("写入验证码失败：%v", err)
	}
	cancelReceived := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orders/"+received.ID+"/cancel", nil)
	cancelReceived.Header.Set("X-HeroMail-User", "admin-001")
	cancelReceived.Header.Set("X-HeroMail-Role", "admin")
	cancelReceivedResponse := httptest.NewRecorder()
	server.Router.ServeHTTP(cancelReceivedResponse, cancelReceived)
	if cancelReceivedResponse.Code != http.StatusConflict {
		t.Fatalf("已收码订单取消返回 %d，期望 %d", cancelReceivedResponse.Code, http.StatusConflict)
	}
	complete := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orders/"+received.ID+"/complete", nil)
	complete.Header.Set("X-HeroMail-User", "admin-001")
	complete.Header.Set("X-HeroMail-Role", "admin")
	completeResponse := httptest.NewRecorder()
	server.Router.ServeHTTP(completeResponse, complete)
	if completeResponse.Code != http.StatusOK {
		t.Fatalf("管理员完成订单返回 %d：%s", completeResponse.Code, completeResponse.Body.String())
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

func TestAdminOrdersFiltersOnServerAndIncludesUserEmail(t *testing.T) {
	repository := store.New()
	adobe, err := repository.CreateOrder("user-001", "svc-adobe", "admin-filter-adobe")
	if err != nil {
		t.Fatalf("创建 Adobe 订单失败：%v", err)
	}
	if _, err := repository.CancelOrder(adobe.ID, "user-001"); err != nil {
		t.Fatalf("取消 Adobe 订单失败：%v", err)
	}
	if _, err := repository.CreateOrder("user-001", "svc-openai", "admin-filter-openai"); err != nil {
		t.Fatalf("创建 OpenAI 订单失败：%v", err)
	}

	server := NewServer(repository)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders?page=1&page_size=20&status=canceled&service=adobe&query=ORD", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	request.Header.Set("X-HeroMail-User", "admin-001")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("筛选管理员订单返回 %d，响应：%s", response.Code, response.Body.String())
	}
	var body struct {
		Data []struct {
			domain.Order
			UserEmail string `json:"user_email"`
		} `json:"data"`
		Pagination struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析管理员订单响应失败：%v", err)
	}
	if len(body.Data) != 1 || body.Pagination.Total != 1 {
		t.Fatalf("管理员订单筛选结果不正确：data=%d total=%d", len(body.Data), body.Pagination.Total)
	}
	if body.Data[0].ID != adobe.ID || body.Data[0].UserEmail != "demo@example.com" {
		t.Fatalf("管理员订单视图不正确：%+v", body.Data[0])
	}
}

func TestUserOrdersFiltersOnServer(t *testing.T) {
	repository := store.New()
	adobe, err := repository.CreateOrder("user-001", "svc-adobe", "user-filter-adobe")
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	if _, err := repository.CancelOrder(adobe.ID, "user-001"); err != nil {
		t.Fatalf("取消订单失败：%v", err)
	}
	server := NewServer(repository)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/orders?page=1&page_size=20&status=canceled&service=adobe&query=ORD", nil)
	request.Header.Set("X-HeroMail-User", "user-001")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("用户订单筛选返回 %d，响应：%s", response.Code, response.Body.String())
	}
	var body struct {
		Data       []domain.Order `json:"data"`
		Pagination struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析用户订单响应失败：%v", err)
	}
	if len(body.Data) != 1 || body.Pagination.Total != 1 || body.Data[0].ID != adobe.ID {
		t.Fatalf("用户订单筛选结果不正确：data=%d total=%d", len(body.Data), body.Pagination.Total)
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

func TestMailboxMessagesDefaultCollapsed(t *testing.T) {
	server := NewServer(store.New())
	request := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("工作台脚本返回 %d，期望 %d", response.Code, http.StatusOK)
	}

	script := response.Body.String()
	if !strings.Contains(script, `<details class="mailbox-message html-mode">`) || !strings.Contains(script, `<summary class="mailbox-message-head">`) {
		t.Fatal("收件箱邮件没有使用默认折叠的语义化结构")
	}
	if !strings.Contains(script, `article.addEventListener("toggle"`) || !strings.Contains(script, `if (!article.open) return;`) {
		t.Fatal("收件箱邮件没有在首次展开时延迟挂载 HTML 正文")
	}
	if !strings.Contains(script, `data-action="order-messages"`) || !strings.Contains(script, `/api/v1/orders/${encodeURIComponent(id)}/messages`) {
		t.Fatal("用户订单没有平台隔离邮件入口")
	}
}

func TestMailboxAdminSupportsNewProvidersAndManualRegistration(t *testing.T) {
	server := NewServer(store.New())
	request := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("工作台脚本返回 %d，期望 %d", response.Code, http.StatusOK)
	}
	script := response.Body.String()
	for _, marker := range []string{`gmail: "Gmail"`, `icloud: "iCloud"`, `mailcom: "Mail.com"`, `data-action="mailbox-registration"`, `data-action="mark-mailbox-service-registered"`, `/services/${encodeURIComponent(serviceID)}/registered`} {
		if !strings.Contains(script, marker) {
			t.Fatalf("邮箱管理页面缺少 %q", marker)
		}
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

func TestUserServicesIncludePriceAndAvailabilityWithoutInternalRules(t *testing.T) {
	server := NewServer(store.New())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services?page=1&page_size=2", nil)
	request.Header.Set("X-HeroMail-User", "user-001")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("用户平台列表返回 %d，响应：%s", response.Code, response.Body.String())
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
		t.Fatalf("解析用户平台列表失败：%v", err)
	}
	if len(body.Data) == 0 || len(body.Data) > 2 || body.Pagination.Page != 1 || body.Pagination.PageSize != 2 {
		t.Fatalf("用户平台分页不正确：data=%d pagination=%+v", len(body.Data), body.Pagination)
	}
	for _, service := range body.Data {
		for _, field := range []string{"code", "name", "description", "allowed_providers", "price", "ttl_seconds", "available_mailboxes"} {
			if _, exists := service[field]; !exists {
				t.Fatalf("用户平台响应缺少字段 %s：%+v", field, service)
			}
		}
		for _, field := range []string{"id", "enabled", "leased_mailboxes", "consumed_mailboxes", "sender_domains", "subject_keywords", "regex"} {
			if _, exists := service[field]; exists {
				t.Fatalf("用户平台响应泄露内部字段 %s：%+v", field, service)
			}
		}
	}
}

func TestServiceAvailabilityByCode(t *testing.T) {
	server := NewServer(store.New())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services/adobe/availability", nil)
	request.Header.Set("X-HeroMail-User", "user-001")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("平台余量接口返回 %d，响应：%s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			Code               string `json:"code"`
			AvailableMailboxes int    `json:"available_mailboxes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析平台余量响应失败：%v", err)
	}
	if body.Data.Code != "adobe" || body.Data.AvailableMailboxes <= 0 {
		t.Fatalf("平台余量响应不正确：%+v", body.Data)
	}

	missing := httptest.NewRequest(http.MethodGet, "/api/v1/services/unknown/availability", nil)
	missing.Header.Set("X-HeroMail-User", "user-001")
	missingResponse := httptest.NewRecorder()
	server.Router.ServeHTTP(missingResponse, missing)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("未知平台余量返回 %d，期望 %d", missingResponse.Code, http.StatusNotFound)
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
	backupCalled := false
	server.UpgradeBackup = func(_ context.Context) (string, error) {
		backupCalled = true
		if _, err := os.Stat(requestPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("备份完成前不应创建升级请求：%v", err)
		}
		return directory + "/backup.sql.gz", nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/upgrade", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("创建升级任务返回 %d，响应：%s", response.Code, response.Body.String())
	}
	if !backupCalled {
		t.Fatal("创建升级任务前没有执行数据库备份")
	}
	if _, err := os.Stat(requestPath); err != nil {
		t.Fatalf("升级请求文件不存在：%v", err)
	}
	status, err := readUpgradeStatus(statusPath)
	if err != nil || status.State != "queued" {
		t.Fatalf("升级状态不正确：%+v，错误：%v", status, err)
	}
}

func TestOnlineUpgradeStopsWhenBackupFails(t *testing.T) {
	directory := t.TempDir()
	requestPath := directory + "/request.json"
	statusPath := directory + "/status.json"
	t.Setenv("HEROMAIL_UPGRADE_REQUEST", requestPath)
	t.Setenv("HEROMAIL_UPGRADE_STATUS", statusPath)
	server := NewServer(store.New())
	server.UpgradeBackup = func(_ context.Context) (string, error) {
		return "", errors.New("备份命令失败")
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/upgrade", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("备份失败返回 %d，期望 %d，响应：%s", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
	if _, err := os.Stat(requestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("备份失败后不应创建升级请求：%v", err)
	}
	status, err := readUpgradeStatus(statusPath)
	if err != nil || status.State != "failed" {
		t.Fatalf("备份失败状态不正确：%+v，错误：%v", status, err)
	}
}

func TestOnlineUpgradeRequiresConfiguredBackup(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HEROMAIL_UPGRADE_REQUEST", directory+"/request.json")
	t.Setenv("HEROMAIL_UPGRADE_STATUS", directory+"/status.json")
	t.Setenv("DATABASE_URL", "")
	server := NewServer(store.New())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/upgrade", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "upgrade_backup_unavailable") {
		t.Fatalf("未配置备份时返回 %d，响应：%s", response.Code, response.Body.String())
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

type mailboxImportRepositoryStub struct {
	store.Repository
	store.ResourceRepository
	saved      []domain.Mailbox
	queued     []string
	enqueueErr error
}

func (s *mailboxImportRepositoryStub) MailboxPoolByName(name string) (domain.MailboxPool, bool) {
	return domain.MailboxPool{Name: name, Provider: "mixed", Enabled: name == domain.DefaultMailboxPoolName}, name == domain.DefaultMailboxPoolName
}

func (s *mailboxImportRepositoryStub) SaveMailbox(_ string, mailbox domain.Mailbox, _ map[string]string, _ string) (domain.Mailbox, error) {
	mailbox.ID = "mailbox-" + mailbox.Address
	s.saved = append(s.saved, mailbox)
	return mailbox, nil
}

func (s *mailboxImportRepositoryStub) EnqueueMailboxVerification(_ context.Context, mailboxID string) error {
	s.queued = append(s.queued, mailboxID)
	return s.enqueueErr
}

func TestAdminImportKeepsSavedMailboxWhenQueueIsTemporarilyUnavailable(t *testing.T) {
	repository := &mailboxImportRepositoryStub{Repository: store.New(), enqueueErr: errors.New("redis unavailable")}
	server := NewServer(repository)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "mailboxes.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("alpha@outlook.com:password\n"))
	_ = writer.Close()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/mailboxes/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-HeroMail-Role", "admin")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(repository.saved) != 1 {
		t.Fatalf("Redis 暂时不可用时导入结果错误：status=%d saved=%d body=%s", response.Code, len(repository.saved), response.Body.String())
	}
}

func (s *mailboxImportRepositoryStub) DequeueMailboxVerification(context.Context, time.Duration) (string, error) {
	return "", nil
}

func TestAdminImportsMailboxFileAndQueuesVerification(t *testing.T) {
	repository := &mailboxImportRepositoryStub{Repository: store.New()}
	server := NewServer(repository)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "mailboxes.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("alpha@outlook.com:password\nbeta@hotmail.com----password\ngamma@outlook.de:password\ndelta@gmail.com:app-password\necho@icloud.com:app-password\nfoxtrot@mail.com:app-password\n"))
	_ = writer.Close()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/mailboxes/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-HeroMail-Role", "admin")
	request.Header.Set("X-HeroMail-User", "admin-001")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("导入邮箱返回 %d，响应：%s", response.Code, response.Body.String())
	}
	if len(repository.saved) != 6 || len(repository.queued) != 6 {
		t.Fatalf("邮箱或验证任务数量错误：saved=%d queued=%d", len(repository.saved), len(repository.queued))
	}
	for _, mailbox := range repository.saved {
		if mailbox.Pool != domain.DefaultMailboxPoolName || mailbox.State != domain.MailboxPending || mailbox.VerificationStatus != domain.MailboxVerificationPending || len(mailbox.RegisteredPlatforms) != 0 {
			t.Fatalf("导入邮箱初始状态错误：%+v", mailbox)
		}
	}
}

func TestAdminMarksMailboxRegisteredForSelectedService(t *testing.T) {
	repository := store.New()
	mailbox, ok := findMailboxByAddress(repository.Mailboxes(), "hero_01@outlook.com")
	if !ok {
		t.Fatal("未找到测试邮箱")
	}
	server := NewServer(repository)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/mailboxes/"+mailbox.ID+"/services/svc-openai/registered", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	request.Header.Set("X-HeroMail-User", "admin-001")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("手工标记返回 %d：%s", response.Code, response.Body.String())
	}
	updated, _ := findMailboxByAddress(repository.Mailboxes(), mailbox.Address)
	if len(updated.RegisteredPlatforms) != 1 || updated.RegisteredPlatforms[0] != "openai" {
		t.Fatalf("已注册平台 = %#v，期望 [openai]", updated.RegisteredPlatforms)
	}
}

func findMailboxByAddress(mailboxes []domain.Mailbox, address string) (domain.Mailbox, bool) {
	for _, mailbox := range mailboxes {
		if mailbox.Address == address {
			return mailbox, true
		}
	}
	return domain.Mailbox{}, false
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
			body:        `{"code":"Grok","name":"grok 注册","enabled":true,"allowed_providers":["outlook","hotmail"],"price":0.02,"ttl_seconds":1800,"sender_domains":[],"subject_keywords":[],"regex":"\\b(\\d{6})\\b"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "至少填写一个发件人域名",
		},
		{
			name:        "缺少主题关键词",
			body:        `{"code":"Grok","name":"grok 注册","enabled":true,"allowed_providers":["outlook","hotmail"],"price":0.02,"ttl_seconds":1800,"sender_domains":["x.ai"],"subject_keywords":[],"regex":"\\b(\\d{6})\\b"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "至少填写一个主题关键词",
		},
		{
			name:        "不支持的邮箱供应商",
			body:        `{"code":"grok","name":"Grok 注册","enabled":true,"allowed_providers":["yahoo"],"price":0.02,"ttl_seconds":1800,"sender_domains":["x.ai"],"subject_keywords":["validate your email"],"regex":"\\b(\\d{6})\\b"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "邮箱类型不受支持",
		},
		{
			name:        "收码时限不是三十分钟",
			body:        `{"code":"Grok","name":"grok 注册","enabled":true,"allowed_providers":["outlook","hotmail"],"price":0.02,"ttl_seconds":600,"sender_domains":["x.ai"],"subject_keywords":["verify"],"regex":"\\b(\\d{6})\\b"}`,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "任务有效期固定为 1800 秒",
		},
		{
			name:       "有效配置",
			body:       `{"code":"Grok","name":"grok 注册","enabled":true,"allowed_providers":["outlook","outlook_de","hotmail","gmail","icloud","mailcom"],"price":0.02,"ttl_seconds":1800,"sender_domains":["x.ai"],"subject_keywords":["validate your email"],"regex":"(?i)\\b([A-Z0-9]{3}-[A-Z0-9]{3}|[A-Z0-9]{6})\\b"}`,
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
	user             domain.User
	changedUserID    string
	currentPassword  string
	newPassword      string
	auditAction      string
	adjustmentActor  string
	adjustmentUser   string
	adjustmentAmount float64
	adjustmentNote   string
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

func (s *accountRepositoryStub) AdjustBalance(actorID, userID string, amount float64, description, _ string) (domain.User, error) {
	s.adjustmentActor = actorID
	s.adjustmentUser = userID
	s.adjustmentAmount = amount
	s.adjustmentNote = description
	return domain.User{ID: userID, Balance: amount}, nil
}

func TestAdminBalanceAdjustmentAllowsEmptyDescription(t *testing.T) {
	repository := &accountRepositoryStub{
		Repository: store.New(),
		user:       domain.User{ID: "admin-001", Email: "admin@example.com", Role: "admin", Status: "active"},
	}
	server := NewServer(repository)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/user-001/balance", bytes.NewBufferString(`{"amount":10}`))
	request.Header.Set("Authorization", "Bearer 有效会话")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("空说明余额调整返回 %d，响应：%s", response.Code, response.Body.String())
	}
	if repository.adjustmentActor != "admin-001" || repository.adjustmentUser != "user-001" || repository.adjustmentAmount != 10 {
		t.Fatalf("余额调整参数不正确：%+v", repository)
	}
	if repository.adjustmentNote != store.DefaultBalanceAdjustmentDescription {
		t.Fatalf("空说明没有使用默认审计文本：%q", repository.adjustmentNote)
	}
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

func validAlipayTestConfig(t *testing.T) map[string]string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成测试支付宝私钥失败：%v", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("序列化测试支付宝私钥失败：%v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("序列化测试支付宝公钥失败：%v", err)
	}
	return map[string]string{
		"app_id":      "2026000000000000",
		"private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
		"public_key":  base64.StdEncoding.EncodeToString(publicDER),
	}
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
	config := validAlipayTestConfig(t)
	repository := &paymentRepositoryStub{
		Repository: store.New(),
		secret: store.PaymentProviderSecret{
			Provider: domain.PaymentProvider{ID: "provider-001", Type: "alipay"},
			Config:   config,
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
	if repository.savedConfig["private_key"] != config["private_key"] || repository.savedConfig["public_key"] != config["public_key"] {
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
