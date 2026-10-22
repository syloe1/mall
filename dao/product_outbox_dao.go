package dao

import "mall/model"

/*
商品事件出库表（可靠消息队列）
保证：商品变更（库存 / 上下架）一定能同步到 ES / 其他服务
*/
type ProductOutboxDB interface {
	//创建一条待发送的商品事件
	Create(event *model.ProductOutbox) error
	//取出【还没发送成功】的事件
	ListPending(limit int) ([]model.ProductOutbox, error)
	//标记事件为【已发送 / 已同步】
	MarkPublished(id uint) error
	//标记事件为【发送失败】
	MarkFailed(id uint, errMsg string) error
}
