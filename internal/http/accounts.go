package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

const currentUserKey = "heromail.current_user"

type credentialsRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=10,max=128"`
	DisplayName string `json:"display_name"`
}

func (s *Server) register(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "registration_unavailable", "注册服务不可用")
		return
	}
	var request credentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "邮箱或密码格式不正确，密码至少 10 位")
		return
	}
	user, token, err := repository.Register(request.Email, request.Password, request.DisplayName)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrEmailExists) {
			status = http.StatusConflict
		}
		writeError(c, status, "registration_failed", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"user": user, "token": token}})
}

func (s *Server) login(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "login_unavailable", "登录服务不可用")
		return
	}
	var request credentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "邮箱或密码格式不正确")
		return
	}
	user, token, err := repository.Login(request.Email, request.Password)
	if err != nil {
		writeError(c, http.StatusUnauthorized, "invalid_credentials", "邮箱或密码错误")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"user": user, "token": token}})
}

func (s *Server) logout(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if ok {
		_ = repository.Logout(bearerToken(c))
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) updateProfile(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "profile_unavailable", "个人设置服务不可用")
		return
	}
	var request struct {
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "请求格式不正确")
		return
	}
	user, err := repository.UpdateProfile(demoUser(c), request.DisplayName)
	if err != nil {
		writeError(c, http.StatusBadRequest, "profile_update_failed", err.Error())
		return
	}
	_ = repository.WriteAudit(user.ID, "user.profile.update", "user", user.ID, "用户更新个人设置", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (s *Server) changePassword(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "password_change_unavailable", "密码修改服务不可用")
		return
	}
	var request struct {
		CurrentPassword string `json:"current_password" binding:"required,max=128"`
		NewPassword     string `json:"new_password" binding:"required,min=10,max=128"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "当前密码不能为空，新密码至少需要 10 位")
		return
	}
	user, _ := authenticatedUser(c)
	token, err := repository.ChangePassword(user.ID, request.CurrentPassword, request.NewPassword)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrPasswordMismatch) || errors.Is(err, store.ErrInvalidCredentials) {
			status = http.StatusUnauthorized
		}
		writeError(c, status, "password_change_failed", err.Error())
		return
	}
	action := "user.password.change"
	detail := "用户修改登录密码并撤销旧会话"
	if user.Role == "admin" {
		action = "admin.password.change"
		detail = "管理员修改登录密码并撤销旧会话"
	}
	_ = repository.WriteAudit(user.ID, action, "user", user.ID, detail, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"token": token}})
}

func (s *Server) listAPIKeys(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "api_keys_unavailable", "API Key 服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListAPIKeysPage(demoUser(c), page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) createAPIKey(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "api_keys_unavailable", "API Key 服务不可用")
		return
	}
	var request struct {
		Name   string   `json:"name" binding:"required"`
		Scopes []string `json:"scopes"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "密钥名称不能为空")
		return
	}
	key, secret, err := repository.CreateAPIKey(demoUser(c), request.Name, request.Scopes)
	if err != nil {
		writeError(c, http.StatusBadRequest, "api_key_create_failed", err.Error())
		return
	}
	_ = repository.WriteAudit(demoUser(c), "api_key.create", "api_key", key.ID, key.Name, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"key": key, "secret": secret}})
}

func (s *Server) revokeAPIKey(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "api_keys_unavailable", "API Key 服务不可用")
		return
	}
	if err := repository.RevokeAPIKey(demoUser(c), c.Param("id")); err != nil {
		writeError(c, http.StatusNotFound, "api_key_not_found", err.Error())
		return
	}
	_ = repository.WriteAudit(demoUser(c), "api_key.revoke", "api_key", c.Param("id"), "用户吊销 API Key", c.ClientIP())
	c.Status(http.StatusNoContent)
}

func (s *Server) walletLedgers(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "wallet_unavailable", "资金流水服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListWalletLedgersPage(demoUser(c), page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) adminUsers(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "users_unavailable", "用户管理服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListUsersPage(page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) adminAdjustBalance(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "wallet_unavailable", "资金服务不可用")
		return
	}
	var request struct {
		Amount      float64 `json:"amount" binding:"required"`
		Description string  `json:"description"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Amount == 0 {
		writeError(c, http.StatusBadRequest, "invalid_request", "调整金额不能为空")
		return
	}
	description := strings.TrimSpace(request.Description)
	if description == "" {
		description = store.DefaultBalanceAdjustmentDescription
	}
	user, err := repository.AdjustBalance(demoUser(c), c.Param("id"), request.Amount, description, c.ClientIP())
	if err != nil {
		writeError(c, http.StatusBadRequest, "balance_adjust_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (s *Server) adminLedgers(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "wallet_unavailable", "资金流水服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListWalletLedgersPage(strings.TrimSpace(c.Query("user_id")), page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) adminAuditLogs(c *gin.Context) {
	repository, ok := s.Store.(store.AccountRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "audit_unavailable", "审计服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListAuditLogsPage(page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		repository, production := s.Store.(store.AccountRepository)
		if !production {
			c.Next()
			return
		}
		token := bearerToken(c)
		user, ok := repository.ResolveAccessToken(token)
		if !ok {
			writeError(c, http.StatusUnauthorized, "authentication_required", "请先登录或提供有效 API Key")
			c.Abort()
			return
		}
		c.Set(currentUserKey, user)
		c.Next()
	}
}

func bearerToken(c *gin.Context) string {
	value := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

func authenticatedUser(c *gin.Context) (domain.User, bool) {
	value, ok := c.Get(currentUserKey)
	if !ok {
		return domain.User{}, false
	}
	user, ok := value.(domain.User)
	return user, ok
}
