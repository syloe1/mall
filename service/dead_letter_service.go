package service

import (
	"mall/dao"
	"mall/model"
)

// 查询死信消息的入参
type ListProductDeadLettersInput struct {
	Page      int    // 页码
	PageSize  int    // 每页条数
	Source    string // 消息来源（如：订单服务、库存服务）
	EventType string // 事件类型（如：创建、更新、删除商品）
	ProductID uint   // 商品ID
}

// 查询死信消息的返回结果
type ListProductDeadLettersResult struct {
	Items    []model.ProductEventDeadLetter `json:"items"`     // 死信记录列表
	Total    int64                          `json:"total"`     // 总条数（分页用）
	Page     int                            `json:"page"`      // 当前页码
	PageSize int                            `json:"page_size"` // 每页条数
}

type DeadLetterService struct {
	productDeadLetterDB dao.ProductDeadLetterDB
}

func NewDeadLetterService(productDeadLetterDB dao.ProductDeadLetterDB) *DeadLetterService {
	return &DeadLetterService{productDeadLetterDB: productDeadLetterDB}
}

func (s *DeadLetterService) ListProductDeadLetters(input ListProductDeadLettersInput) (*ListProductDeadLettersResult, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}

	records, total, err := s.productDeadLetterDB.List(dao.ProductDeadLetterQuery{
		Page:      input.Page,
		PageSize:  input.PageSize,
		Source:    input.Source,
		EventType: input.EventType,
		ProductID: input.ProductID,
	})
	if err != nil {
		return nil, err
	}

	return &ListProductDeadLettersResult{
		Items:    records,
		Total:    total,
		Page:     input.Page,
		PageSize: input.PageSize,
	}, nil
}
