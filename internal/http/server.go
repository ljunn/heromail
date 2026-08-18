package httpapi

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ljunn/heromail/internal/buildinfo"
	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/mail"
	"github.com/ljunn/heromail/internal/payment"
	"github.com/ljunn/heromail/internal/store"
	"github.com/ljunn/heromail/internal/web"
)

type Server struct {
	Store              store.Repository
	Router             *gin.Engine
	UpgradeRequestPath string
	UpgradeStatusPath  string
	Payment            *payment.Service
	Microsoft          *mail.MicrosoftClient
	PublicURL          string
	WorkerToken        string
}

func NewServer(st store.Repository) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), cors())
	s := &Server{
		Store:              st,
		Router:             r,
		UpgradeRequestPath: os.Getenv("HEROMAIL_UPGRADE_REQUEST"),
		UpgradeStatusPath:  os.Getenv("HEROMAIL_UPGRADE_STATUS"),
		PublicURL:          os.Getenv("HEROMAIL_PUBLIC_URL"),
		WorkerToken:        os.Getenv("HEROMAIL_WORKER_TOKEN"),
	}
	redirectURI := os.Getenv("MICROSOFT_REDIRECT_URI")
	if redirectURI == "" && s.PublicURL != "" {
		redirectURI = strings.TrimRight(s.PublicURL, "/") + "/api/v1/oauth/microsoft/callback"
	}
	s.Microsoft = mail.NewMicrosoftClient(mail.MicrosoftConfig{ClientID: os.Getenv("MICROSOFT_CLIENT_ID"), ClientSecret: os.Getenv("MICROSOFT_CLIENT_SECRET"), Tenant: os.Getenv("MICROSOFT_TENANT"), RedirectURI: redirectURI})
	if repository, ok := st.(store.PaymentRepository); ok {
		s.Payment = payment.New(repository, os.Getenv("HEROMAIL_PUBLIC_URL"))
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	staticFS, err := fs.Sub(web.Files, "static")
	if err != nil {
		panic(err)
	}
	assetVersion := buildinfo.Commit
	if assetVersion == "" || assetVersion == "unknown" {
		assetVersion = buildinfo.Version
	}
	readEntry := func(name string) []byte {
		content, readErr := fs.ReadFile(web.Files, "static/"+name)
		if readErr != nil {
			panic(readErr)
		}
		return []byte(strings.ReplaceAll(string(content), "__HEROMAIL_ASSET_VERSION__", assetVersion))
	}
	publicHTML := readEntry("index.html")
	workspaceHTML := readEntry("workspace.html")
	serveEntry := func(content []byte) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Header("Cache-Control", "no-store")
			c.Data(http.StatusOK, "text/html; charset=utf-8", content)
		}
	}
	for _, path := range []string{"/", "/pricing", "/docs", "/open-source", "/login", "/register"} {
		s.Router.GET(path, serveEntry(publicHTML))
	}
	s.Router.GET("/status", func(c *gin.Context) { c.Redirect(http.StatusFound, "/") })
	s.Router.GET("/docs/*path", serveEntry(publicHTML))
	s.Router.GET("/app", serveEntry(workspaceHTML))
	s.Router.GET("/app/*path", serveEntry(workspaceHTML))
	s.Router.GET("/admin", serveEntry(workspaceHTML))
	s.Router.GET("/admin/*path", serveEntry(workspaceHTML))
	staticHandler := http.FileServer(http.FS(staticFS))
	s.Router.NoRoute(gin.WrapH(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		staticHandler.ServeHTTP(w, r)
	})))
	s.Router.GET("/favicon.ico", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	s.Router.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	s.Router.GET("/readyz", s.ready)

	api := s.Router.Group("/api/v1")
	api.POST("/auth/register", s.register)
	api.POST("/auth/login", s.login)
	api.GET("/public/services", s.publicServices)
	api.GET("/public/status", s.publicStatus)
	api.POST("/internal/mail-events", s.receiveMailEvent)
	api.GET("/oauth/microsoft/callback", s.microsoftOAuthCallback)
	api.GET("/payment/webhook/:type", s.paymentWebhook)
	api.POST("/payment/webhook/:type", s.paymentWebhook)

	protected := api.Group("", s.authenticate())
	protected.POST("/auth/logout", s.logout)
	protected.GET("/me", s.me)
	protected.PUT("/me", s.updateProfile)
	protected.PUT("/me/password", s.changePassword)
	protected.GET("/services", s.services)
	protected.GET("/orders", s.listUserOrders)
	protected.POST("/orders", s.createOrder)
	protected.GET("/orders/:id", s.getUserOrder)
	protected.POST("/orders/:id/submitted", s.submitOrder)
	protected.POST("/orders/:id/complete", s.completeOrder)
	protected.POST("/orders/:id/cancel", s.cancelOrder)
	protected.GET("/api-keys", s.listAPIKeys)
	protected.POST("/api-keys", s.createAPIKey)
	protected.DELETE("/api-keys/:id", s.revokeAPIKey)
	protected.GET("/wallet/ledgers", s.walletLedgers)
	protected.GET("/webhooks", s.listWebhookEndpoints)
	protected.POST("/webhooks", s.createWebhookEndpoint)
	protected.DELETE("/webhooks/:id", s.deleteWebhookEndpoint)
	protected.GET("/webhook-deliveries", s.listWebhookDeliveries)
	protected.POST("/webhook-deliveries/:id/retry", s.retryWebhookDelivery)
	protected.GET("/payment/methods", s.paymentMethods)
	protected.POST("/payment/orders", s.createPaymentOrder)
	protected.GET("/payment/orders", s.paymentOrders)
	protected.GET("/payment/orders/:id", s.getPaymentOrder)
	protected.POST("/payment/orders/:id/cancel", s.cancelPaymentOrder)

	admin := protected.Group("/admin", requireAdmin())
	admin.GET("/overview", s.adminOverview)
	admin.GET("/mailboxes", s.adminMailboxes)
	admin.POST("/mailboxes", s.adminSaveMailbox)
	admin.DELETE("/mailboxes/:id", s.adminDeleteMailbox)
	admin.GET("/mailbox-pools", s.adminMailboxPools)
	admin.POST("/mailbox-pools", s.adminSaveMailboxPool)
	admin.DELETE("/mailbox-pools/:id", s.adminDeleteMailboxPool)
	admin.POST("/mailboxes/oauth/microsoft", s.microsoftOAuthStart)
	admin.GET("/services", s.adminServices)
	admin.POST("/services", s.adminSaveService)
	admin.DELETE("/services/:id", s.adminDeleteService)
	admin.GET("/orders", s.adminOrders)
	admin.GET("/users", s.adminUsers)
	admin.POST("/users/:id/balance", s.adminAdjustBalance)
	admin.GET("/wallet/ledgers", s.adminLedgers)
	admin.GET("/audit-logs", s.adminAuditLogs)
	admin.GET("/payment/providers", s.adminPaymentProviders)
	admin.GET("/payment/providers/:id", s.adminPaymentProvider)
	admin.POST("/payment/providers", s.adminSavePaymentProvider)
	admin.DELETE("/payment/providers/:id", s.adminDeletePaymentProvider)
	admin.GET("/payment/orders", s.adminPaymentOrders)
	admin.GET("/system/version", s.adminVersion)
	admin.POST("/system/upgrade", s.adminUpgrade)
	admin.POST("/reap", s.adminReap)
}

func (s *Server) me(c *gin.Context) {
	if user, ok := authenticatedUser(c); ok {
		c.JSON(http.StatusOK, user)
		return
	}
	userID := demoUser(c)
	user, ok := s.Store.User(userID)
	if !ok {
		writeError(c, http.StatusUnauthorized, "user_not_found", "user not found")
		return
	}
	c.JSON(http.StatusOK, user)
}

func (s *Server) services(c *gin.Context) {
	page, pageSize := pageRequest(c)
	items, total := s.Store.ListServicesPage(page, pageSize)
	writePage(c, serviceViews(s.Store, items), page, pageSize, total)
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
	page, pageSize := pageRequest(c)
	items, total := s.Store.ListOrdersPage(demoUser(c), page, pageSize)
	writePage(c, items, page, pageSize, total)
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
	if _, production := s.Store.(store.ResourceRepository); !production {
		s.scheduleDemoCode(order.ID)
	}
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
	page, pageSize := pageRequest(c)
	items, total := s.Store.MailboxesPage(page, pageSize)
	writePage(c, items, page, pageSize, total)
}

type serviceAdminView struct {
	domain.Service
	AvailableMailboxes int `json:"available_mailboxes"`
	LeasedMailboxes    int `json:"leased_mailboxes"`
	ConsumedMailboxes  int `json:"consumed_mailboxes"`
}

func (s *Server) adminServices(c *gin.Context) {
	page, pageSize := pageRequest(c)
	services, total := s.Store.ListServicesPage(page, pageSize)
	writePage(c, serviceViews(s.Store, services), page, pageSize, total)
}

func serviceViews(repository store.Repository, services []domain.Service) []serviceAdminView {
	serviceIDs := make([]string, 0, len(services))
	for _, service := range services {
		serviceIDs = append(serviceIDs, service.ID)
	}
	usage := repository.ServiceUsage(serviceIDs)
	views := make([]serviceAdminView, 0)
	for _, service := range services {
		counts := usage[service.ID]
		view := serviceAdminView{Service: service, AvailableMailboxes: counts.Available, LeasedMailboxes: counts.Leased, ConsumedMailboxes: counts.Consumed}
		views = append(views, view)
	}
	return views
}

func (s *Server) adminOrders(c *gin.Context) {
	page, pageSize := pageRequest(c)
	items, total := s.Store.ListOrdersPage("", page, pageSize)
	writePage(c, items, page, pageSize, total)
}

func (s *Server) ready(c *gin.Context) {
	health, ok := s.Store.(store.HealthRepository)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	if err := health.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "storage": health.StorageName()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready", "storage": health.StorageName()})
}

func pageRequest(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func writePage(c *gin.Context, data any, page, pageSize int, total int64) {
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	c.JSON(http.StatusOK, gin.H{
		"data":       data,
		"pagination": gin.H{"page": page, "page_size": pageSize, "total": total, "total_pages": totalPages},
	})
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
	if user, ok := authenticatedUser(c); ok {
		return user.ID
	}
	if id := c.GetHeader("X-HeroMail-User"); id != "" {
		return id
	}
	if demoRole(c) == "admin" {
		return "admin-001"
	}
	return "user-001"
}
func demoRole(c *gin.Context) string {
	if user, ok := authenticatedUser(c); ok {
		return user.Role
	}
	role := strings.ToLower(c.GetHeader("X-HeroMail-Role"))
	if role == "admin" {
		return role
	}
	return "user"
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-HeroMail-Role, X-HeroMail-User, X-HeroMail-Target-Version")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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
