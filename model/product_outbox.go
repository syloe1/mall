package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	ProductEventCreated = "product.created"
	ProductEventUpdated = "product.updated"
	ProductEventDeleted = "product.deleted"
)

const (
	ProductOutboxPending   = "pending"
	ProductOutboxPublished = "published"
	ProductOutboxFailed    = "failed"
)

type ProductOutbox struct {
	gorm.Model

	EventID     string     `json:"event_id" gorm:"size:64;uniqueIndex;not null"`
	EventType   string     `json:"event_type" gorm:"size:64;index;not null"`
	ProductID   uint       `json:"product_id" gorm:"index;not null"`
	Status      string     `json:"status" gorm:"size:32;index;not null"`
	RetryCount  int        `json:"retry_count" gorm:"not null;default:0"`
	LastError   string     `json:"last_error" gorm:"size:512"`
	PublishedAt *time.Time `json:"published_at"`
}
