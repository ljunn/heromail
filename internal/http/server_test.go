package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestOnlineUpgradeRequiresIndependentToken(t *testing.T) {
	directory := t.TempDir()
	requestPath := directory + "/request.json"
	statusPath := directory + "/status.json"
	t.Setenv("HEROMAIL_UPGRADE_REQUEST", requestPath)
	t.Setenv("HEROMAIL_UPGRADE_STATUS", statusPath)
	t.Setenv("HEROMAIL_UPDATE_TOKEN", "update-test-token")
	server := NewServer(store.New())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/upgrade", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	request.Header.Set("X-HeroMail-Update-Token", "wrong-token")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("错误升级令牌返回 %d，期望 %d", response.Code, http.StatusForbidden)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/upgrade", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	request.Header.Set("X-HeroMail-Update-Token", "update-test-token")
	response = httptest.NewRecorder()
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
	t.Setenv("HEROMAIL_UPDATE_TOKEN", "update-test-token")
	originalVersion := buildinfo.Version
	buildinfo.Version = "v1.0.3"
	t.Cleanup(func() { buildinfo.Version = originalVersion })
	server := NewServer(store.New())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/upgrade", nil)
	request.Header.Set("X-HeroMail-Role", "admin")
	request.Header.Set("X-HeroMail-Update-Token", "update-test-token")
	request.Header.Set("X-HeroMail-Target-Version", "v1.0.3")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("同版本升级返回 %d，期望 %d，响应：%s", response.Code, http.StatusConflict, response.Body.String())
	}
}
