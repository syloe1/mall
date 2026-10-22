package dao

import "mall/model"

/*
存处理失败的商品消息（扣库存 / 更新失败）
方便后续重试、排查错误。
*/
type ProductDeadLetterQuery struct {
	Page      int    // 分页页码
	PageSize  int    // 每页条数
	Source    string // 来源：比如 order / payment
	EventType string // 事件类型：stock_decrease / update
	ProductID uint   // 商品ID
}

type ProductDeadLetterDB interface {
	//创建一条失败消息
	Create(record *model.ProductEventDeadLetter) error
	//后台查看死信消息列表
	List(query ProductDeadLetterQuery) ([]model.ProductEventDeadLetter, int64, error)
}
