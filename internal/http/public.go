package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type publicServiceView struct {
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	AllowedProviders []string `json:"allowed_providers"`
	Price            float64  `json:"price"`
	TTLSeconds       int      `json:"ttl_seconds"`
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
			Price:            service.Price,
			TTLSeconds:       service.TTLSeconds,
		})
	}
	writePage(c, items, page, pageSize, total)
}

func (s *Server) publicStatus(c *gin.Context) {
	status := "operational"
	if response := s.Store.Overview(); response.AuthErrors > 0 {
		status = "degraded"
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"status":      status,
		"api":         "operational",
		"allocation":  "operational",
		"mail_worker": "operational",
		"updated_at":  time.Now().UTC(),
	}})
}
