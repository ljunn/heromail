package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ljunn/heromail/internal/store"
)

type Worker struct {
	repository   store.WebhookRepository
	client       *http.Client
	interval     time.Duration
	allowPrivate bool
}

func NewWorker(repository store.WebhookRepository, allowPrivate bool) *Worker {
	worker := &Worker{repository: repository, interval: 5 * time.Second, allowPrivate: allowPrivate}
	worker.client = &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{DialContext: worker.safeDialContext, TLSHandshakeTimeout: 8 * time.Second}}
	return worker
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.deliver(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.deliver(ctx)
		}
	}
}

func (w *Worker) deliver(ctx context.Context) {
	jobs, err := w.repository.ClaimWebhookJobs(20)
	if err != nil {
		return
	}
	for _, job := range jobs {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, []byte(job.Secret))
		mac.Write([]byte(timestamp + "."))
		mac.Write(job.Payload)
		signature := "t=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, job.URL, bytes.NewReader(job.Payload))
		if requestErr != nil {
			_ = w.repository.FailWebhookJob(job.Delivery.ID, 0, requestErr.Error())
			continue
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("User-Agent", "HeroMail-Webhook/1.0")
		request.Header.Set("X-HeroMail-Event", job.Delivery.Event)
		request.Header.Set("X-HeroMail-Delivery", job.Delivery.ID)
		request.Header.Set("X-HeroMail-Signature", signature)
		response, responseErr := w.client.Do(request)
		if responseErr != nil {
			_ = w.repository.FailWebhookJob(job.Delivery.ID, 0, responseErr.Error())
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		response.Body.Close()
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			_ = w.repository.CompleteWebhookJob(job.Delivery.ID, response.StatusCode)
		} else {
			_ = w.repository.FailWebhookJob(job.Delivery.ID, response.StatusCode, fmt.Sprintf("Webhook 返回 HTTP %d", response.StatusCode))
		}
	}
}

func (w *Worker) safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, fmt.Errorf("无法解析 Webhook 地址：%s", host)
	}
	for _, item := range addresses {
		if !w.allowPrivate && isPrivate(item.IP) {
			return nil, fmt.Errorf("Webhook 禁止访问内网地址：%s", host)
		}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
}

func isPrivate(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || strings.HasPrefix(ip.String(), "169.254.")
}
