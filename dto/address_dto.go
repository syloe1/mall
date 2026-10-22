package dto

type AddressCreateReq struct {
	ReceiverName string `json:"receiver_name" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Province     string `json:"province" binding:"required"`
	City         string `json:"city" binding:"required"`
	District     string `json:"district" binding:"required"`
	Detail       string `json:"detail" binding:"required"`
	PostalCode   string `json:"postal_code"`
	IsDefault    bool   `json:"is_default"`
}

type AddressUpdateReq struct {
	ID           uint   `json:"id" binding:"required"` //ID：地址 ID （必填，要知道改哪个地址）
	ReceiverName string `json:"receiver_name" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Province     string `json:"province" binding:"required"`
	City         string `json:"city" binding:"required"`
	District     string `json:"district" binding:"required"`
	Detail       string `json:"detail" binding:"required"`
	PostalCode   string `json:"postal_code"`
	IsDefault    bool   `json:"is_default"`
}

type AddressDeleteReq struct {
	ID uint `json:"id" binding:"required"`
}

type AddressGetReq struct {
	ID uint `form:"id" json:"id" binding:"required"`
}

type AddressSetDefaultReq struct {
	ID uint `json:"id" binding:"required"`
}
