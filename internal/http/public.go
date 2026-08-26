package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type publicServiceView struct {
	Code             string             `json:"code"`
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	AllowedProviders []string           `json:"allowed_providers"`
	ProviderPrices   map[string]float64 `json:"provider_prices"`
	TTLSeconds       int                `json:"ttl_seconds"`
}

func (s *Server) publicServices(c *gin.Context) {
	page, pageSize := pageRequest(c)
	services, total := s.Store.ListEnabledServicesPage(page, pageSize)
	items := make([]publicServiceView, 0, len(services))
	for _, service := range services {
		items = append(items, publicServiceView{
			Code:             service.Code,
			Name:             service.Name,
			Description:      service.Description,
			AllowedProviders: append([]string(nil), service.AllowedProviders...),
			ProviderPrices:   copyProviderPrices(service.ProviderPrices),
			TTLSeconds:       service.TTLSeconds,
		})
	}
	writePage(c, items, page, pageSize, total)
}

func copyProviderPrices(prices map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(prices))
	for provider, price := range prices {
		result[provider] = price
	}
	return result
}

func (s *Server) publicStatus(c *gin.Context) {
	status := "operational"
	overview := s.Store.Overview()
	mailWorkerStatus := "operational"
	if overview.AuthErrors > 0 || overview.PendingMailboxes > 0 {
		status = "degraded"
		mailWorkerStatus = "degraded"
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"status":      status,
		"api":         "operational",
		"allocation":  "operational",
		"mail_worker": mailWorkerStatus,
		"updated_at":  time.Now().UTC(),
	}})
}
