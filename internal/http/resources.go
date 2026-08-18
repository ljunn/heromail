package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

func (s *Server) adminMailboxPools(c *gin.Context) {
	repository, ok := s.Store.(store.ResourceRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "mailbox_pools_unavailable", "邮箱池服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListMailboxPoolsPage(page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) adminSaveMailboxPool(c *gin.Context) {
	repository, ok := s.Store.(store.ResourceRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "mailbox_pools_unavailable", "邮箱池服务不可用")
		return
	}
	var request domain.MailboxPool
	if err := c.ShouldBindJSON(&request); err != nil || request.Name == "" || request.Provider == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "邮箱池名称和供应商不能为空")
		return
	}
	pool, err := repository.SaveMailboxPool(demoUser(c), request, c.ClientIP())
	if err != nil {
		writeError(c, http.StatusBadRequest, "mailbox_pool_save_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": pool})
}

func (s *Server) adminDeleteMailboxPool(c *gin.Context) {
	repository, ok := s.Store.(store.ResourceRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "mailbox_pools_unavailable", "邮箱池服务不可用")
		return
	}
	if err := repository.DeleteMailboxPool(demoUser(c), c.Param("id"), c.ClientIP()); err != nil {
		writeError(c, http.StatusConflict, "mailbox_pool_delete_failed", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) adminSaveMailbox(c *gin.Context) {
	repository, ok := s.Store.(store.ResourceRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "mailboxes_unavailable", "邮箱资源服务不可用")
		return
	}
	var request struct {
		domain.Mailbox
		Credential map[string]string `json:"credential"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Address == "" || request.Provider == "" || request.Pool == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "邮箱地址、供应商和邮箱池不能为空")
		return
	}
	mailbox, err := repository.SaveMailbox(demoUser(c), request.Mailbox, request.Credential, c.ClientIP())
	if err != nil {
		writeError(c, http.StatusBadRequest, "mailbox_save_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": mailbox})
}

func (s *Server) adminDeleteMailbox(c *gin.Context) {
	repository, ok := s.Store.(store.ResourceRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "mailboxes_unavailable", "邮箱资源服务不可用")
		return
	}
	if err := repository.DeleteMailbox(demoUser(c), c.Param("id"), c.ClientIP()); err != nil {
		writeError(c, http.StatusConflict, "mailbox_delete_failed", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) adminSaveService(c *gin.Context) {
	repository, ok := s.Store.(store.ResourceRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "services_unavailable", "目标平台服务不可用")
		return
	}
	var request domain.Service
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "目标平台配置格式无效")
		return
	}
	request.Code = strings.ToLower(strings.TrimSpace(request.Code))
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.Regex = strings.TrimSpace(request.Regex)
	request.AllowedProviders = normalizeServiceList(request.AllowedProviders)
	request.SenderDomains = normalizeServiceList(request.SenderDomains)
	request.SubjectKeywords = normalizeServiceList(request.SubjectKeywords)
	if request.Code == "" || request.Name == "" || request.Regex == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "平台代码、名称和验证码正则不能为空")
		return
	}
	if _, err := regexp.Compile(request.Regex); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_code_regex", "验证码正则表达式无效")
		return
	}
	if request.Price < 0 {
		writeError(c, http.StatusBadRequest, "invalid_service_config", "单价不能小于 0")
		return
	}
	if request.TTLSeconds < 60 || request.TTLSeconds > 86400 {
		writeError(c, http.StatusBadRequest, "invalid_service_config", "有效期必须在 60 到 86400 秒之间")
		return
	}
	if len(request.AllowedProviders) == 0 {
		writeError(c, http.StatusBadRequest, "invalid_service_config", "至少选择一个允许的邮箱供应商")
		return
	}
	for _, provider := range request.AllowedProviders {
		if provider != "outlook" && provider != "hotmail" {
			writeError(c, http.StatusBadRequest, "invalid_service_config", "邮箱供应商只允许 outlook 和 hotmail")
			return
		}
	}
	if len(request.SenderDomains) == 0 {
		writeError(c, http.StatusBadRequest, "invalid_service_config", "至少填写一个发件人域名")
		return
	}
	service, err := repository.SaveService(demoUser(c), request, c.ClientIP())
	if err != nil {
		writeError(c, http.StatusBadRequest, "service_save_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": service})
}

func normalizeServiceList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *Server) adminDeleteService(c *gin.Context) {
	repository, ok := s.Store.(store.ResourceRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "services_unavailable", "目标平台服务不可用")
		return
	}
	if err := repository.DeleteService(demoUser(c), c.Param("id"), c.ClientIP()); err != nil {
		writeError(c, http.StatusConflict, "service_delete_failed", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) microsoftOAuthStart(c *gin.Context) {
	repository, ok := s.Store.(store.ResourceRepository)
	if !ok || s.Microsoft == nil || !s.Microsoft.Enabled() {
		writeError(c, http.StatusServiceUnavailable, "microsoft_oauth_unavailable", "Microsoft OAuth 尚未配置")
		return
	}
	var request struct {
		Pool string `json:"pool" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "请选择邮箱池")
		return
	}
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		writeError(c, http.StatusInternalServerError, "oauth_state_failed", "无法创建 OAuth 状态")
		return
	}
	state := base64.RawURLEncoding.EncodeToString(buffer)
	if err := repository.CreateOAuthState(state, store.OAuthState{ActorID: demoUser(c), Pool: request.Pool}, 10*time.Minute); err != nil {
		writeError(c, http.StatusInternalServerError, "oauth_state_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"authorization_url": s.Microsoft.AuthURL(state)}})
}

func (s *Server) microsoftOAuthCallback(c *gin.Context) {
	repository, ok := s.Store.(store.ResourceRepository)
	if !ok || s.Microsoft == nil || !s.Microsoft.Enabled() {
		writeError(c, http.StatusServiceUnavailable, "microsoft_oauth_unavailable", "Microsoft OAuth 尚未配置")
		return
	}
	state, err := repository.ConsumeOAuthState(c.Query("state"))
	if err != nil || c.Query("code") == "" {
		writeError(c, http.StatusBadRequest, "invalid_oauth_state", "OAuth 状态无效或已过期")
		return
	}
	credential, validUntil, err := s.Microsoft.Exchange(c.Request.Context(), c.Query("code"))
	if err != nil {
		writeError(c, http.StatusBadGateway, "oauth_exchange_failed", err.Error())
		return
	}
	profile, err := s.Microsoft.Profile(c.Request.Context(), credential["access_token"])
	if err != nil {
		writeError(c, http.StatusBadGateway, "oauth_profile_failed", err.Error())
		return
	}
	provider := "outlook"
	if strings.Contains(profile.Address, "@hotmail.") || strings.HasSuffix(profile.Address, "@hotmail.com") {
		provider = "hotmail"
	}
	_, err = repository.SaveMailbox(state.ActorID, domain.Mailbox{Address: profile.Address, Provider: provider, Pool: state.Pool, State: domain.MailboxAvailable, HealthScore: 100, OAuthValidUntil: validUntil}, credential, c.ClientIP())
	if err != nil {
		writeError(c, http.StatusBadRequest, "mailbox_save_failed", err.Error())
		return
	}
	redirect := "/?oauth=microsoft&status=success"
	if s.PublicURL != "" {
		redirect = strings.TrimRight(s.PublicURL, "/") + "/?oauth=microsoft&status=success&address=" + url.QueryEscape(profile.Address)
	}
	c.Redirect(http.StatusFound, redirect)
}
