package model

import "gorm.io/gorm"

const (
	ProductDeadLetterPublisher = "publisher"
	ProductDeadLetterConsumer  = "consumer"
)

type ProductEventDeadLetter struct {
	gorm.Model

	EventID      string `json:"event_id" gorm:"size:64;index;not null"`
	EventType    string `json:"event_type" gorm:"size:64;index;not null"`
	ProductID    uint   `json:"product_id" gorm:"index;not null"`
	Source       string `json:"source" gorm:"size:32;index;not null"`
	RetryCount   int    `json:"retry_count" gorm:"not null;default:0"`
	ErrorMessage string `json:"error_message" gorm:"size:1024;not null"`
	Payload      string `json:"payload" gorm:"type:text"`
}
