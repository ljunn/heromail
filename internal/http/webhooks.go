package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ljunn/heromail/internal/store"
)

func (s *Server) listWebhookEndpoints(c *gin.Context) {
	repository, ok := s.Store.(store.WebhookRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "webhooks_unavailable", "Webhook 服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListWebhookEndpointsPage(demoUser(c), page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) createWebhookEndpoint(c *gin.Context) {
	repository, ok := s.Store.(store.WebhookRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "webhooks_unavailable", "Webhook 服务不可用")
		return
	}
	var request struct {
		URL    string   `json:"url" binding:"required,url"`
		Events []string `json:"events"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Webhook URL 格式不正确")
		return
	}
	endpoint, secret, err := repository.CreateWebhookEndpoint(demoUser(c), request.URL, request.Events)
	if err != nil {
		writeError(c, http.StatusBadRequest, "webhook_create_failed", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"endpoint": endpoint, "secret": secret}})
}

func (s *Server) deleteWebhookEndpoint(c *gin.Context) {
	repository, ok := s.Store.(store.WebhookRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "webhooks_unavailable", "Webhook 服务不可用")
		return
	}
	if err := repository.DeleteWebhookEndpoint(demoUser(c), c.Param("id")); err != nil {
		writeError(c, http.StatusNotFound, "webhook_not_found", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) listWebhookDeliveries(c *gin.Context) {
	repository, ok := s.Store.(store.WebhookRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "webhooks_unavailable", "Webhook 服务不可用")
		return
	}
	page, pageSize := pageRequest(c)
	items, total := repository.ListWebhookDeliveriesPage(demoUser(c), page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) retryWebhookDelivery(c *gin.Context) {
	repository, ok := s.Store.(store.WebhookRepository)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "webhooks_unavailable", "Webhook 服务不可用")
		return
	}
	if err := repository.RetryWebhookJob(demoUser(c), c.Param("id")); err != nil {
		writeError(c, http.StatusNotFound, "webhook_delivery_not_found", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
