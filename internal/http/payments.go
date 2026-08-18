package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

func (s *Server) paymentMethods(c *gin.Context) {
	if s.Payment == nil {
		writeError(c, http.StatusServiceUnavailable, "payment_unavailable", "支付服务不可用")
		return
	}
	methods, err := s.Payment.Methods()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "payment_methods_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": methods})
}

func (s *Server) createPaymentOrder(c *gin.Context) {
	if s.Payment == nil {
		writeError(c, http.StatusServiceUnavailable, "payment_unavailable", "支付服务不可用")
		return
	}
	var request struct {
		Amount float64 `json:"amount" binding:"required"`
		Method string  `json:"method" binding:"required"`
		Mobile bool    `json:"mobile"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "支付金额和方式不能为空")
		return
	}
	order, err := s.Payment.Create(demoUser(c), request.Amount, request.Method, request.Mobile)
	if err != nil {
		writeError(c, http.StatusBadRequest, "payment_create_failed", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": order})
}

func (s *Server) paymentOrders(c *gin.Context) {
	repository, ok := s.Store.(store.PaymentRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "payment_unavailable", "支付服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListPaymentOrdersPage(demoUser(c), page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) getPaymentOrder(c *gin.Context) {
	repository, ok := s.Store.(store.PaymentRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "payment_unavailable", "支付服务不可用")
		return
	}
	order, exists := repository.GetPaymentOrder(demoUser(c), c.Param("id"))
	if !exists {
		writeError(c, http.StatusNotFound, "payment_order_not_found", "支付订单不存在")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
}

func (s *Server) cancelPaymentOrder(c *gin.Context) {
	repository, ok := s.Store.(store.PaymentRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "payment_unavailable", "支付服务不可用")
		return
	}
	order, err := repository.CancelPaymentOrder(demoUser(c), c.Param("id"))
	if err != nil {
		writeError(c, http.StatusConflict, "payment_cancel_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
}

func (s *Server) paymentWebhook(c *gin.Context) {
	if s.Payment == nil {
		c.String(http.StatusServiceUnavailable, "fail")
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	if err := s.Payment.Notify(c.Param("type"), c.Query("provider_id"), c.Request.Form); err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	c.String(http.StatusOK, "success")
}

func (s *Server) adminPaymentProviders(c *gin.Context) {
	repository, ok := s.Store.(store.PaymentRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "payment_unavailable", "支付服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListPaymentProvidersPage(page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) adminPaymentProvider(c *gin.Context) {
	repository, ok := s.Store.(store.PaymentRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "payment_unavailable", "支付服务不可用")
		return
	}
	secret, err := repository.GetPaymentProviderSecret(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusNotFound, "payment_provider_not_found", err.Error())
		return
	}
	config := map[string]string{}
	configured := map[string]bool{}
	for _, key := range []string{"api_base", "pid", "channel_id", "gateway", "app_id"} {
		config[key] = secret.Config[key]
	}
	for _, key := range []string{"pkey", "private_key", "public_key"} {
		configured[key] = strings.TrimSpace(secret.Config[key]) != ""
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"provider": secret.Provider, "config": config, "configured": configured}})
}

func (s *Server) adminSavePaymentProvider(c *gin.Context) {
	repository, ok := s.Store.(store.PaymentRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "payment_unavailable", "支付服务不可用")
		return
	}
	var request struct {
		ID       string            `json:"id"`
		Name     string            `json:"name" binding:"required"`
		Type     string            `json:"type" binding:"required,oneof=easypay alipay"`
		Methods  []string          `json:"methods" binding:"required"`
		Enabled  bool              `json:"enabled"`
		Priority int               `json:"priority"`
		Config   map[string]string `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "支付服务商配置不完整")
		return
	}
	if request.ID != "" {
		existing, err := repository.GetPaymentProviderSecret(request.ID)
		if err != nil {
			writeError(c, http.StatusNotFound, "payment_provider_not_found", err.Error())
			return
		}
		if existing.Provider.Type != request.Type {
			writeError(c, http.StatusBadRequest, "payment_provider_type_locked", "编辑服务商时不能修改接入类型")
			return
		}
		for _, key := range []string{"pkey", "private_key", "public_key"} {
			value := existing.Config[key]
			if strings.TrimSpace(request.Config[key]) == "" {
				request.Config[key] = value
			}
		}
	}
	if request.Type == "easypay" {
		if strings.TrimSpace(request.Config["api_base"]) == "" || strings.TrimSpace(request.Config["pid"]) == "" || strings.TrimSpace(request.Config["pkey"]) == "" {
			writeError(c, http.StatusBadRequest, "invalid_request", "易支付必须填写 API 地址、商户 ID 和商户密钥")
			return
		}
	}
	if request.Type == "alipay" {
		request.Config["gateway"] = "https://openapi.alipay.com/gateway.do"
		if strings.TrimSpace(request.Config["app_id"]) == "" || strings.TrimSpace(request.Config["private_key"]) == "" || strings.TrimSpace(request.Config["public_key"]) == "" {
			writeError(c, http.StatusBadRequest, "invalid_request", "支付宝官方必须填写 AppID、应用私钥和支付宝公钥")
			return
		}
	}
	provider, err := repository.SavePaymentProvider(demoUser(c), domain.PaymentProvider{ID: request.ID, Name: request.Name, Type: request.Type, Methods: request.Methods, Enabled: request.Enabled, Priority: request.Priority}, request.Config, c.ClientIP())
	if err != nil {
		writeError(c, http.StatusBadRequest, "payment_provider_save_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": provider})
}

func (s *Server) adminDeletePaymentProvider(c *gin.Context) {
	repository, ok := s.Store.(store.PaymentRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "payment_unavailable", "支付服务不可用")
		return
	}
	if err := repository.DeletePaymentProvider(demoUser(c), c.Param("id"), c.ClientIP()); err != nil {
		if errors.Is(err, store.ErrPaymentProviderInUse) {
			writeError(c, http.StatusConflict, "payment_provider_in_use", err.Error())
			return
		}
		writeError(c, http.StatusNotFound, "payment_provider_not_found", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) adminPaymentOrders(c *gin.Context) {
	repository, ok := s.Store.(store.PaymentRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "payment_unavailable", "支付服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListPaymentOrdersPage("", page, pageSize)
	writePage(c, items, page, pageSize, total)
}
