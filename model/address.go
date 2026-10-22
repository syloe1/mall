package model

import "gorm.io/gorm"

type UserAddress struct {
	gorm.Model
	UserID       uint   `json:"user_id" gorm:"index;not null"`
	ReceiverName string `json:"receiver_name" gorm:"size:64;not null"`
	Phone        string `json:"phone" gorm:"size:11;not null"`
	Province     string `json:"province" gorm:"size:64;not null"`
	City         string `json:"city" gorm:"size:64;not null"`
	District     string `json:"district" gorm:"size:64;not null"`
	Detail       string `json:"detail" gorm:"size:255;not null"`
	PostalCode   string `json:"postal_code" gorm:"size:16"`
	IsDefault    bool   `json:"is_default" gorm:"not null;default:false"`
}
