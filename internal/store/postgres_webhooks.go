package store

import (
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/ljunn/heromail/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *PostgresStore) CreateWebhookEndpoint(userID, endpointURL string, events []string) (domain.WebhookEndpoint, string, error) {
	parsed, err := url.Parse(endpointURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return domain.WebhookEndpoint{}, "", errors.New("Webhook URL 无效")
	}
	if len(events) == 0 {
		events = []string{"order.code_received", "order.completed", "order.canceled", "order.expired_refunded"}
	}
	secret, err := randomToken("whsec_", 32)
	if err != nil {
		return domain.WebhookEndpoint{}, "", err
	}
	encrypted, err := s.encryptJSON(map[string]string{"secret": secret})
	if err != nil {
		return domain.WebhookEndpoint{}, "", err
	}
	row := sqlWebhookEndpoint{ID: uuid.NewString(), UserID: userID, URL: endpointURL, Events: events, Enabled: true, EncryptedSecret: encrypted}
	if err := s.db.Create(&row).Error; err != nil {
		return domain.WebhookEndpoint{}, "", err
	}
	return mapWebhookEndpoint(row), secret, nil
}

func (s *PostgresStore) ListWebhookEndpointsPage(userID string, page, pageSize int) ([]domain.WebhookEndpoint, int64) {
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.Model(&sqlWebhookEndpoint{}).Where("user_id = ?", userID)
	var total int64
	query.Count(&total)
	var rows []sqlWebhookEndpoint
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.WebhookEndpoint, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapWebhookEndpoint(row))
	}
	return items, total
}

func (s *PostgresStore) DeleteWebhookEndpoint(userID, endpointID string) error {
	result := s.db.Delete(&sqlWebhookEndpoint{}, "id = ? AND user_id = ?", endpointID, userID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("Webhook 端点不存在")
	}
	return nil
}

func (s *PostgresStore) ListWebhookDeliveriesPage(userID string, page, pageSize int) ([]domain.WebhookDelivery, int64) {
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.Table("webhook_deliveries AS d").Joins("JOIN webhook_endpoints AS e ON e.id = d.endpoint_id").Where("e.user_id = ?", userID)
	var total int64
	query.Count(&total)
	var rows []sqlWebhookDelivery
	query.Select("d.*").Order("d.created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows)
	items := make([]domain.WebhookDelivery, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapWebhookDelivery(row))
	}
	return items, total
}

func (s *PostgresStore) ClaimWebhookJobs(limit int) ([]WebhookJob, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	jobs := make([]WebhookJob, 0)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var deliveries []sqlWebhookDelivery
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status IN ? AND next_retry_at <= ?", []string{"pending", "retrying"}, time.Now()).Order("next_retry_at ASC").Limit(limit).Find(&deliveries).Error; err != nil {
			return err
		}
		for _, delivery := range deliveries {
			var endpoint sqlWebhookEndpoint
			if err := tx.First(&endpoint, "id = ? AND enabled = ?", delivery.EndpointID, true).Error; err != nil {
				tx.Model(&delivery).Updates(map[string]any{"status": "failed", "last_error": "Webhook 端点已停用或删除"})
				continue
			}
			secretConfig := make(map[string]string)
			if err := s.decryptJSON(endpoint.EncryptedSecret, &secretConfig); err != nil {
				return err
			}
			delivery.Status = "processing"
			delivery.Attempts++
			if err := tx.Save(&delivery).Error; err != nil {
				return err
			}
			jobs = append(jobs, WebhookJob{Delivery: mapWebhookDelivery(delivery), URL: endpoint.URL, Secret: secretConfig["secret"], Payload: []byte(delivery.Payload)})
		}
		return nil
	})
	return jobs, err
}

func (s *PostgresStore) CompleteWebhookJob(deliveryID string, responseCode int) error {
	now := time.Now().UTC()
	return s.db.Model(&sqlWebhookDelivery{}).Where("id = ?", deliveryID).Updates(map[string]any{"status": "delivered", "response_code": responseCode, "last_error": "", "delivered_at": now}).Error
}

func (s *PostgresStore) FailWebhookJob(deliveryID string, responseCode int, message string) error {
	var row sqlWebhookDelivery
	if err := s.db.First(&row, "id = ?", deliveryID).Error; err != nil {
		return err
	}
	status := "retrying"
	delays := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour}
	if row.Attempts >= len(delays) {
		status = "failed"
	}
	delayIndex := row.Attempts - 1
	if delayIndex < 0 {
		delayIndex = 0
	}
	if delayIndex >= len(delays) {
		delayIndex = len(delays) - 1
	}
	return s.db.Model(&row).Updates(map[string]any{"status": status, "response_code": responseCode, "last_error": message, "next_retry_at": time.Now().Add(delays[delayIndex])}).Error
}

func (s *PostgresStore) RetryWebhookJob(userID, deliveryID string) error {
	result := s.db.Table("webhook_deliveries AS d").Where("d.id = ? AND EXISTS (SELECT 1 FROM webhook_endpoints e WHERE e.id = d.endpoint_id AND e.user_id = ?)", deliveryID, userID).Updates(map[string]any{"status": "retrying", "next_retry_at": time.Now(), "last_error": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("Webhook 投递记录不存在")
	}
	return nil
}

func (s *PostgresStore) enqueueOrderWebhook(tx *gorm.DB, order sqlOrder, event string) error {
	var endpoints []sqlWebhookEndpoint
	if err := tx.Where("user_id = ? AND enabled = ?", order.UserID, true).Find(&endpoints).Error; err != nil {
		return err
	}
	for _, endpoint := range endpoints {
		if !containsString(endpoint.Events, event) && !containsString(endpoint.Events, "*") {
			continue
		}
		deliveryID := uuid.NewString()
		payload, err := json.Marshal(map[string]any{"id": deliveryID, "event": event, "created_at": time.Now().UTC(), "data": map[string]any{"order": mapOrder(order)}})
		if err != nil {
			return err
		}
		delivery := sqlWebhookDelivery{ID: deliveryID, EndpointID: endpoint.ID, OrderID: order.ID, Event: event, Status: "pending", Payload: string(payload), NextRetryAt: time.Now().UTC()}
		if err := tx.Create(&delivery).Error; err != nil {
			return err
		}
	}
	return nil
}

func mapWebhookEndpoint(row sqlWebhookEndpoint) domain.WebhookEndpoint {
	return domain.WebhookEndpoint{ID: row.ID, URL: row.URL, Events: append([]string(nil), row.Events...), Enabled: row.Enabled, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func mapWebhookDelivery(row sqlWebhookDelivery) domain.WebhookDelivery {
	return domain.WebhookDelivery{ID: row.ID, EndpointID: row.EndpointID, OrderID: row.OrderID, Event: row.Event, Status: row.Status, Attempts: row.Attempts, ResponseCode: row.ResponseCode, LastError: row.LastError, NextRetryAt: row.NextRetryAt, DeliveredAt: row.DeliveredAt, CreatedAt: row.CreatedAt}
}
