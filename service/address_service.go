package service

import (
	"errors"
	"mall/dao"
	"mall/model"
	"mall/pkg/core"
	"net/http"
	"strings"

	"gorm.io/gorm"
)

const maxAddressCount = 20

type AddressService struct {
	db dao.AddressDB
}

func NewAddressService(db dao.AddressDB) *AddressService {
	return &AddressService{
		db: db,
	}
}

type CreateAddressInput struct {
	UserID       uint
	ReceiverName string
	Phone        string
	Province     string
	City         string
	District     string
	Detail       string
	PostalCode   string
	IsDefault    bool
}

type UpdateAddressInput struct {
	UserID       uint
	AddressID    uint
	ReceiverName string
	Phone        string
	Province     string
	City         string
	District     string
	Detail       string
	PostalCode   string
	IsDefault    bool
}

type GetAddressInput struct {
	UserID    uint
	AddressID uint
}

type DeleteAddressInput struct {
	UserID    uint
	AddressID uint
}

type SetDefaultAddressInput struct {
	UserID    uint
	AddressID uint
}

func (s *AddressService) Create(input CreateAddressInput) error {
	if err := validateAddressOwner(input.UserID); err != nil {
		return err
	}

	receiverName, phone, province, city, district, detail := normalizeAddressFields(
		input.ReceiverName,
		input.Phone,
		input.Province,
		input.City,
		input.District,
		input.Detail,
	)

	if err := validateAddressFields(receiverName, phone, province, city, district, detail); err != nil {
		return err
	}

	count, err := s.db.CountByUserID(input.UserID)
	if err != nil {
		return err
	}
	if count >= maxAddressCount {
		return core.NewBizError(http.StatusBadRequest, "收货地址数量已达上限")
	}

	isDefault := input.IsDefault || count == 0
	if isDefault {
		if err := s.db.ClearDefault(input.UserID); err != nil {
			return err
		}
	}

	address := &model.UserAddress{
		UserID:       input.UserID,
		ReceiverName: receiverName,
		Phone:        phone,
		Province:     province,
		City:         city,
		District:     district,
		Detail:       detail,
		PostalCode:   strings.TrimSpace(input.PostalCode),
		IsDefault:    isDefault,
	}

	return s.db.Create(address)
}
func (s *AddressService) List(userID uint) ([]model.UserAddress, error) {
	if err := validateAddressOwner(userID); err != nil {
		return nil, err
	}

	return s.db.ListByUserID(userID)
}
func (s *AddressService) Get(input GetAddressInput) (*model.UserAddress, error) {
	if err := validateAddressOwner(input.UserID); err != nil {
		return nil, err
	}
	if input.AddressID == 0 {
		return nil, core.NewBizError(http.StatusBadRequest, "地址 ID 不合法")
	}
	/*
		select * from user_address where user_id = ? and id = ？
	*/
	address, err := s.db.GetByID(input.UserID, input.AddressID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.NewBizError(http.StatusNotFound, "收货地址不存在")
		}
		return nil, err
	}

	return address, nil
}
func (s *AddressService) Update(input UpdateAddressInput) error {
	if err := validateAddressOwner(input.UserID); err != nil {
		return err
	}
	if input.AddressID == 0 {
		return core.NewBizError(http.StatusBadRequest, "地址 ID 不合法")
	}

	receiverName, phone, province, city, district, detail := normalizeAddressFields(
		input.ReceiverName,
		input.Phone,
		input.Province,
		input.City,
		input.District,
		input.Detail,
	)

	if err := validateAddressFields(receiverName, phone, province, city, district, detail); err != nil {
		return err
	}

	address, err := s.db.GetByID(input.UserID, input.AddressID)
	if err != nil {
		//数据库没发现
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return core.NewBizError(http.StatusNotFound, "收货地址不存在")
		}
		//其他数据库错误
		return err
	}

	if input.IsDefault {
		if err := s.db.ClearDefault(input.UserID); err != nil {
			return err
		}
	}

	address.ReceiverName = receiverName
	address.Phone = phone
	address.Province = province
	address.City = city
	address.District = district
	address.Detail = detail
	address.PostalCode = strings.TrimSpace(input.PostalCode)
	address.IsDefault = input.IsDefault

	return s.db.Update(address)
}

func (s *AddressService) Delete(input DeleteAddressInput) error {
	if err := validateAddressOwner(input.UserID); err != nil {
		return err
	}
	if input.AddressID == 0 {
		return core.NewBizError(http.StatusBadRequest, "地址 ID 不合法")
	}
	//delete from user_address where id = ? and user_id = ?
	rowsAffected, err := s.db.Delete(input.UserID, input.AddressID)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return core.NewBizError(http.StatusNotFound, "收货地址不存在")
	}

	return nil
}

/*
给某个用户，把指定的地址设置为默认地址。
先把该用户所有地址都取消默认
再把指定地址设为默认
*/
func (s *AddressService) SetDefault(input SetDefaultAddressInput) error {
	if err := validateAddressOwner(input.UserID); err != nil {
		return err
	}
	if input.AddressID == 0 {
		return core.NewBizError(http.StatusBadRequest, "地址 ID 不合法")
	}

	if _, err := s.db.GetByID(input.UserID, input.AddressID); err != nil {
		//数据库找不到
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return core.NewBizError(http.StatusNotFound, "收货地址不存在")
		}
		//其他错误
		return err
	}
	//所有地址取消默认
	if err := s.db.ClearDefault(input.UserID); err != nil {
		return err
	}
	//gorm执行update/delete时，告诉你到底改了几条数据
	rowsAffected, err := s.db.SetDefault(input.UserID, input.AddressID)
	if err != nil {
		return err
	}
	//高并发下的安全校验
	if rowsAffected == 0 {
		return core.NewBizError(http.StatusNotFound, "收货地址不存在")
	}

	return nil
}

func validateAddressOwner(userID uint) error {
	if userID == 0 {
		return core.NewBizError(http.StatusBadRequest, "用户 ID 不合法")
	}
	return nil
}

func normalizeAddressFields(receiverName, phone, province, city, district, detail string) (string, string, string, string, string, string) {
	return strings.TrimSpace(receiverName),
		strings.TrimSpace(phone),
		strings.TrimSpace(province),
		strings.TrimSpace(city),
		strings.TrimSpace(district),
		strings.TrimSpace(detail)
}

func validateAddressFields(receiverName, phone, province, city, district, detail string) error {
	if receiverName == "" {
		return core.NewBizError(http.StatusBadRequest, "请填写收货人姓名")
	}
	if phone == "" {
		return core.NewBizError(http.StatusBadRequest, "请填写手机号")
	}
	if province == "" {
		return core.NewBizError(http.StatusBadRequest, "请选择省份")
	}
	if city == "" {
		return core.NewBizError(http.StatusBadRequest, "请选择城市")
	}
	if district == "" {
		return core.NewBizError(http.StatusBadRequest, "请选择区县")
	}
	if detail == "" {
		return core.NewBizError(http.StatusBadRequest, "请填写详细地址")
	}
	return nil
}
