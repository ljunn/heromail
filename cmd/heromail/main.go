package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	httpapi "github.com/ljunn/heromail/internal/http"
	"github.com/ljunn/heromail/internal/mail"
	"github.com/ljunn/heromail/internal/store"
	"github.com/ljunn/heromail/internal/webhook"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	var repository store.Repository
	if strings.EqualFold(os.Getenv("HEROMAIL_STORAGE"), "memory") {
		log.Print("警告：正在使用仅限测试的内存存储")
		repository = store.New()
	} else {
		postgresStore, err := store.NewPostgresStore(ctx, store.PostgresConfig{
			DSN:           os.Getenv("DATABASE_URL"),
			RedisAddress:  os.Getenv("REDIS_ADDR"),
			RedisPassword: os.Getenv("REDIS_PASSWORD"),
			AdminEmail:    os.Getenv("HEROMAIL_ADMIN_EMAIL"),
			AdminPassword: os.Getenv("HEROMAIL_ADMIN_PASSWORD"),
			EncryptionKey: os.Getenv("HEROMAIL_ENCRYPTION_KEY"),
			SeedDemo:      strings.EqualFold(os.Getenv("HEROMAIL_SEED_DEMO"), "true"),
		})
		if err != nil {
			log.Fatalf("初始化 PostgreSQL 和 Redis 失败：%v", err)
		}
		repository = postgresStore
	}
	server := httpapi.NewServer(repository)
	if resources, ok := repository.(store.ResourceRepository); ok && server.Microsoft != nil && server.Microsoft.Enabled() {
		if receiver, receiverOK := repository.(mail.CodeReceiver); receiverOK {
			go mail.NewWorker(resources, receiver, server.Microsoft, 15*time.Second).Run(ctx)
		}
	}
	if webhooks, ok := repository.(store.WebhookRepository); ok {
		go webhook.NewWorker(webhooks, strings.EqualFold(os.Getenv("HEROMAIL_WEBHOOK_ALLOW_PRIVATE"), "true")).Run(ctx)
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				repository.ReapExpired()
				if payments, ok := repository.(store.PaymentRepository); ok {
					payments.ReapExpiredPaymentOrders()
				}
			}
		}
	}()
	httpServer := &http.Server{Addr: ":" + port, Handler: server.Router, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("HeroMail 正在监听 :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务异常退出：%v", err)
		}
	}()
	<-ctx.Done()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		log.Printf("HTTP 服务关闭失败：%v", err)
	}
}
