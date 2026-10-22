package model

import "gorm.io/gorm"

const (
	RoleSuperAdmin = "superadmin"
	RoleAdmin      = "admin"
	RoleUser       = "user"

	DefaultAvator = "/static/imgs/avatar/default.png"
)

type User struct {
	gorm.Model
	Username      string `gorm:"unique;not null" json:"username"`
	Nickname      string `json:"nickname"`
	Avatar        string `json:"avatar"`
	Password      string `json:"-"`
	PayPassword   string `json:"-"`
	Role          string `json:"role"`
	Amount        string `json:"amount"`
}
