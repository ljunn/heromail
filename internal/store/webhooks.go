package store

import "github.com/ljunn/heromail/internal/domain"

type WebhookJob struct {
	Delivery domain.WebhookDelivery
	URL      string
	Secret   string
	Payload  []byte
}

type WebhookRepository interface {
	CreateWebhookEndpoint(userID, endpointURL string, events []string) (domain.WebhookEndpoint, string, error)
	ListWebhookEndpointsPage(userID string, page, pageSize int) ([]domain.WebhookEndpoint, int64)
	DeleteWebhookEndpoint(userID, endpointID string) error
	ListWebhookDeliveriesPage(userID string, page, pageSize int) ([]domain.WebhookDelivery, int64)
	ClaimWebhookJobs(limit int) ([]WebhookJob, error)
	CompleteWebhookJob(deliveryID string, responseCode int) error
	FailWebhookJob(deliveryID string, responseCode int, message string) error
	RetryWebhookJob(userID, deliveryID string) error
}
