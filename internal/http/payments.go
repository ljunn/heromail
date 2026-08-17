package httpapi

import (
	"net/http"

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
