package httpapi

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ljunn/heromail/internal/mail"
	"github.com/ljunn/heromail/internal/store"
)

func (s *Server) receiveMailEvent(c *gin.Context) {
	if s.WorkerToken == "" || subtle.ConstantTimeCompare([]byte(c.GetHeader("X-HeroMail-Worker-Token")), []byte(s.WorkerToken)) != 1 {
		writeError(c, http.StatusForbidden, "invalid_worker_token", "Worker Token 无效")
		return
	}
	receiver, ok := s.Store.(mail.CodeReceiver)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "mail_worker_unavailable", "收码状态机不可用")
		return
	}
	var request struct {
		OrderID string `json:"order_id" binding:"required"`
		Code    string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "订单号和验证码不能为空")
		return
	}
	order, err := receiver.ReceiveCodeValue(request.OrderID, request.Code)
	if err != nil {
		status, code := mapStoreError(err)
		writeError(c, status, code, err.Error())
		return
	}
	if repository, ok := s.Store.(store.AccountRepository); ok {
		_ = repository.WriteAudit("mail-worker", "order.code.receive", "registration_order", order.ID, "邮件 Worker 写入验证码", c.ClientIP())
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
}
