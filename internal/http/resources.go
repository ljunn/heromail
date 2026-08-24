package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/mail"
	"github.com/ljunn/heromail/internal/store"
)

const maxMailboxImportBytes = 100 << 20

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
	if err := c.ShouldBindJSON(&request); err != nil || request.Address == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "邮箱地址不能为空")
		return
	}
	provider, supported := domain.DetectMailboxProvider(request.Address)
	if !supported {
		writeError(c, http.StatusBadRequest, "unsupported_mailbox_provider", "首版只支持 Outlook/Hotmail 邮箱")
		return
	}
	request.Provider = provider
	request.Pool = domain.DefaultMailboxPoolName
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

func (s *Server) adminVerifyMailbox(c *gin.Context) {
	if s.MailboxVerifier == nil {
		writeError(c, http.StatusServiceUnavailable, "mailbox_verification_unavailable", "邮箱验证服务不可用")
		return
	}
	result, err := s.MailboxVerifier.Verify(c.Request.Context(), demoUser(c), c.Param("id"), c.ClientIP())
	if err != nil {
		writeError(c, http.StatusBadGateway, "mailbox_verification_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (s *Server) adminMailboxMessages(c *gin.Context) {
	if s.MailboxVerifier == nil {
		writeError(c, http.StatusServiceUnavailable, "mailbox_messages_unavailable", "邮箱收件服务不可用")
		return
	}
	messages, err := s.MailboxVerifier.ReadMessages(c.Request.Context(), demoUser(c), c.Param("id"), c.ClientIP())
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, store.ErrMailboxNotFound) {
			status = http.StatusNotFound
		}
		writeError(c, status, "mailbox_messages_failed", err.Error())
		return
	}
	page, pageSize := pageRequest(c)
	total := int64(len(messages))
	start := (page - 1) * pageSize
	if start >= len(messages) {
		messages = []mail.Message{}
	} else {
		end := start + pageSize
		if end > len(messages) {
			end = len(messages)
		}
		messages = messages[start:end]
	}
	if audit, ok := s.Store.(store.AuditRepository); ok {
		_ = audit.WriteAudit(demoUser(c), "mailbox.messages.read", "mailbox", c.Param("id"), "管理员查看邮箱收件列表", c.ClientIP())
	}
	writePage(c, messages, page, pageSize, total)
}

func (s *Server) userOrderMessages(c *gin.Context) {
	userID := demoUser(c)
	order, ok := s.Store.GetOrder(c.Param("id"))
	if !ok || order.UserID != userID {
		writeError(c, http.StatusNotFound, "order_not_found", "order not found")
		return
	}
	if s.MailboxVerifier == nil {
		writeError(c, http.StatusServiceUnavailable, "order_messages_unavailable", "订单邮件暂时不可用")
		return
	}
	var service domain.Service
	for _, candidate := range s.Store.ListServices() {
		if candidate.ID == order.ServiceID || candidate.Code == order.ServiceCode {
			service = candidate
			break
		}
	}
	if service.ID == "" {
		writeError(c, http.StatusNotFound, "service_not_found", "目标平台不存在")
		return
	}
	messages, err := s.MailboxVerifier.ReadMessages(c.Request.Context(), userID, order.MailboxID, c.ClientIP())
	if err != nil {
		writeError(c, http.StatusBadGateway, "order_messages_failed", "暂时无法读取该订单邮件")
		return
	}
	messages = mail.FilterMessagesForOrder(service, order, messages)
	page, pageSize := pageRequest(c)
	total := int64(len(messages))
	start := (page - 1) * pageSize
	if start >= len(messages) {
		messages = []mail.Message{}
	} else {
		end := start + pageSize
		if end > len(messages) {
			end = len(messages)
		}
		messages = messages[start:end]
	}
	if audit, ok := s.Store.(store.AuditRepository); ok {
		_ = audit.WriteAudit(userID, "order.messages.read", "registration_order", order.ID, "用户查看订单对应平台邮件", c.ClientIP())
	}
	writePage(c, messages, page, pageSize, total)
}

func (s *Server) adminImportMailboxes(c *gin.Context) {
	repository, ok := s.Store.(store.ResourceRepository)
	queue, queueOK := s.Store.(store.MailboxVerificationQueue)
	if !ok || !queueOK {
		writeError(c, http.StatusServiceUnavailable, "mailbox_import_unavailable", "邮箱导入服务不可用")
		return
	}
	poolName := domain.DefaultMailboxPoolName
	pool, exists := repository.MailboxPoolByName(poolName)
	if !exists || !pool.Enabled {
		writeError(c, http.StatusBadRequest, "mailbox_pool_not_found", "系统邮箱池不可用")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMailboxImportBytes)
	multipartReader, err := c.Request.MultipartReader()
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_mailbox_file", "请上传 TXT 或 CSV 文件")
		return
	}
	var result mail.MailboxImportResult
	foundFile := false
	for {
		part, nextErr := multipartReader.NextPart()
		if nextErr != nil {
			if errors.Is(nextErr, io.EOF) {
				break
			}
			writeError(c, http.StatusBadRequest, "mailbox_file_read_failed", "读取上传文件失败")
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		foundFile = true
		result, err = mail.StreamMailboxImport(part, mail.NewMailboxLineParser(), func(record mail.MailboxImportRecord) error {
			mailbox, saveErr := repository.SaveMailbox(demoUser(c), domain.Mailbox{
				Address:             record.Address,
				Provider:            record.Provider,
				Pool:                pool.Name,
				State:               domain.MailboxPending,
				ConnectionMethod:    domain.MailboxConnectionAuto,
				VerificationStatus:  domain.MailboxVerificationPending,
				RegisteredPlatforms: []string{},
			}, record.Credential(), c.ClientIP())
			if saveErr != nil {
				return saveErr
			}
			// 入队失败时由验证 Worker 的周期补偿扫描重新加入，邮箱已成功导入。
			_ = queue.EnqueueMailboxVerification(c.Request.Context(), mailbox.ID)
			return nil
		})
		_ = part.Close()
		break
	}
	if !foundFile {
		writeError(c, http.StatusBadRequest, "mailbox_file_missing", "请选择要导入的 TXT 或 CSV 文件")
		return
	}
	if err != nil {
		writeError(c, http.StatusBadRequest, "mailbox_import_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
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
	if request.TTLSeconds != domain.MinimumOrderTTLSeconds {
		writeError(c, http.StatusBadRequest, "invalid_service_config", "任务有效期固定为 1800 秒")
		return
	}
	if len(request.AllowedProviders) == 0 {
		writeError(c, http.StatusBadRequest, "invalid_service_config", "至少选择一个允许的邮箱供应商")
		return
	}
	for _, provider := range request.AllowedProviders {
		if !domain.IsSupportedMailboxProvider(provider) {
			writeError(c, http.StatusBadRequest, "invalid_service_config", "邮箱类型只允许 outlook、outlook_de 和 hotmail")
			return
		}
	}
	if len(request.SenderDomains) == 0 {
		writeError(c, http.StatusBadRequest, "invalid_service_config", "至少填写一个发件人域名")
		return
	}
	if len(request.SubjectKeywords) == 0 {
		writeError(c, http.StatusBadRequest, "invalid_service_config", "至少填写一个主题关键词")
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
		Pool string `json:"pool"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "请求格式无效")
		return
	}
	request.Pool = domain.DefaultMailboxPoolName
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
	queue, queueOK := s.Store.(store.MailboxVerificationQueue)
	if !ok || !queueOK || s.Microsoft == nil || !s.Microsoft.Enabled() {
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
	provider, supported := domain.DetectMailboxProvider(profile.Address)
	if !supported {
		writeError(c, http.StatusBadRequest, "unsupported_mailbox_provider", "首版只支持 Outlook/Hotmail 邮箱")
		return
	}
	mailbox, err := repository.SaveMailbox(state.ActorID, domain.Mailbox{
		Address:             profile.Address,
		Provider:            provider,
		Pool:                domain.DefaultMailboxPoolName,
		State:               domain.MailboxPending,
		OAuthValidUntil:     validUntil,
		ConnectionMethod:    domain.MailboxConnectionAuto,
		VerificationStatus:  domain.MailboxVerificationPending,
		RegisteredPlatforms: []string{},
	}, credential, c.ClientIP())
	if err != nil {
		writeError(c, http.StatusBadRequest, "mailbox_save_failed", err.Error())
		return
	}
	_ = queue.EnqueueMailboxVerification(c.Request.Context(), mailbox.ID)
	redirect := "/?oauth=microsoft&status=success"
	if s.PublicURL != "" {
		redirect = strings.TrimRight(s.PublicURL, "/") + "/?oauth=microsoft&status=success&address=" + url.QueryEscape(profile.Address)
	}
	c.Redirect(http.StatusFound, redirect)
}
