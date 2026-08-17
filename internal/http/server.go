package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

type Server struct {
	Store  *store.Store
	Router *gin.Engine
}

func NewServer(st *store.Store) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), cors())
	s := &Server{Store: st, Router: r}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Router.GET("/", func(c *gin.Context) { c.File("web/index.html") })
	s.Router.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	s.Router.GET("/readyz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ready", "storage": "memory"}) })

	api := s.Router.Group("/api/v1")
	api.GET("/me", s.me)
	api.GET("/services", s.services)
	api.GET("/orders", s.listUserOrders)
	api.POST("/orders", s.createOrder)
	api.GET("/orders/:id", s.getUserOrder)
	api.POST("/orders/:id/submitted", s.submitOrder)
	api.POST("/orders/:id/complete", s.completeOrder)
	api.POST("/orders/:id/cancel", s.cancelOrder)

	admin := api.Group("/admin", requireAdmin())
	admin.GET("/overview", s.adminOverview)
	admin.GET("/mailboxes", s.adminMailboxes)
	admin.GET("/services", s.adminServices)
	admin.GET("/orders", s.adminOrders)
	admin.POST("/reap", s.adminReap)
}

func (s *Server) me(c *gin.Context) {
	userID := demoUser(c)
	user, ok := s.Store.User(userID)
	if !ok {
		writeError(c, http.StatusUnauthorized, "user_not_found", "user not found")
		return
	}
	c.JSON(http.StatusOK, user)
}

func (s *Server) services(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": s.Store.ListServices()})
}

type createOrderRequest struct {
	Service   string `json:"service" binding:"required"`
	RequestID string `json:"request_id"`
}

func (s *Server) createOrder(c *gin.Context) {
	var request createOrderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "service is required")
		return
	}
	serviceID := request.Service
	for _, service := range s.Store.ListServices() {
		if service.ID == request.Service || service.Code == request.Service {
			serviceID = service.ID
			break
		}
	}
	order, err := s.Store.CreateOrder(demoUser(c), serviceID, request.RequestID)
	if err != nil {
		status, code := mapStoreError(err)
		writeError(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": order})
}

func (s *Server) listUserOrders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": s.Store.ListOrders(demoUser(c))})
}

func (s *Server) getUserOrder(c *gin.Context) {
	order, ok := s.Store.GetOrder(c.Param("id"))
	if !ok || order.UserID != demoUser(c) {
		writeError(c, http.StatusNotFound, "order_not_found", "order not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
}

func (s *Server) submitOrder(c *gin.Context) {
	order, err := s.Store.SubmitOrder(c.Param("id"), demoUser(c))
	if err != nil {
		status, code := mapStoreError(err)
		writeError(c, status, code, err.Error())
		return
	}
	s.scheduleDemoCode(order.ID)
	c.JSON(http.StatusOK, gin.H{"data": order})
}

func (s *Server) completeOrder(c *gin.Context) {
	order, err := s.Store.CompleteOrder(c.Param("id"), demoUser(c))
	if err != nil {
		status, code := mapStoreError(err)
		writeError(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
}

func (s *Server) cancelOrder(c *gin.Context) {
	order, err := s.Store.CancelOrder(c.Param("id"), demoUser(c))
	if err != nil {
		status, code := mapStoreError(err)
		writeError(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
}

func (s *Server) adminOverview(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": s.Store.Overview()})
}

func (s *Server) adminMailboxes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": s.Store.Mailboxes()})
}

type serviceAdminView struct {
	domain.Service
	AvailableMailboxes int `json:"available_mailboxes"`
	LeasedMailboxes    int `json:"leased_mailboxes"`
	ConsumedMailboxes  int `json:"consumed_mailboxes"`
}

func (s *Server) adminServices(c *gin.Context) {
	mailboxes := s.Store.Mailboxes()
	views := make([]serviceAdminView, 0)
	for _, service := range s.Store.ListServices() {
		view := serviceAdminView{Service: service}
		for _, mailbox := range mailboxes {
			state := mailbox.Services[service.ID].State
			switch state {
			case domain.ServiceAvailable:
				view.AvailableMailboxes++
			case domain.ServiceLeased:
				view.LeasedMailboxes++
			case domain.ServiceConsumed:
				view.ConsumedMailboxes++
			}
		}
		views = append(views, view)
	}
	c.JSON(http.StatusOK, gin.H{"data": views})
}

func (s *Server) adminOrders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": s.Store.ListOrders("")})
}

func (s *Server) adminReap(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"reaped": s.Store.ReapExpired()})
}

func (s *Server) scheduleDemoCode(orderID string) {
	time.AfterFunc(1500*time.Millisecond, func() { _, _ = s.Store.ReceiveCode(orderID) })
}

func requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if demoRole(c) != "admin" {
			writeError(c, http.StatusForbidden, "admin_required", "admin role required")
			c.Abort()
			return
		}
		c.Next()
	}
}

func demoUser(c *gin.Context) string {
	if id := c.GetHeader("X-HeroMail-User"); id != "" {
		return id
	}
	if demoRole(c) == "admin" {
		return "admin-001"
	}
	return "user-001"
}
func demoRole(c *gin.Context) string {
	role := strings.ToLower(c.GetHeader("X-HeroMail-Role"))
	if role == "admin" {
		return role
	}
	return "user"
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-HeroMail-Role, X-HeroMail-User")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code, "message": message})
}

func mapStoreError(err error) (int, string) {
	switch {
	case errors.Is(err, store.ErrServiceNotFound):
		return http.StatusNotFound, "service_not_found"
	case errors.Is(err, store.ErrServiceDisabled):
		return http.StatusConflict, "service_disabled"
	case errors.Is(err, store.ErrNoMailboxAvailable):
		return http.StatusServiceUnavailable, "no_mailbox_available"
	case errors.Is(err, store.ErrInsufficientBalance):
		return http.StatusPaymentRequired, "insufficient_balance"
	case errors.Is(err, store.ErrDuplicateRequest):
		return http.StatusConflict, "duplicate_request"
	case errors.Is(err, store.ErrOrderNotFound):
		return http.StatusNotFound, "order_not_found"
	case errors.Is(err, store.ErrInvalidOrderState):
		return http.StatusConflict, "invalid_order_state"
	default:
		return http.StatusBadRequest, "request_failed"
	}
}
