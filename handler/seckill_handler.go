package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"mall/dto"
	"mall/pkg/core"
	"mall/service"
)

type SeckillHandler struct {
	svc *service.SeckillService
}

func NewSeckillHandler(svc *service.SeckillService) *SeckillHandler {
	return &SeckillHandler{svc: svc}
}

// @Summary      秒杀抢购
// @Description  参与指定活动的秒杀，需要 JWT 认证，接口有令牌桶限流；活动已结束返回 410，未开始返回 409
// @Tags         秒杀
// @Accept       json
// @Produce      json
// @Param        activity_id  path  uint  true  "秒杀活动ID"
// @Success      200  {object}  core.Response  "{\"status\":\"queued\"}"
// @Failure      400  {object}  core.Response
// @Failure      401  {object}  core.Response
// @Failure      409  {object}  core.Response  "活动未开始 / 已抢光 / 重复抢购"
// @Failure      410  {object}  core.Response  "活动已结束"
// @Failure      429  {object}  core.Response  "请求过于频繁"
// @Security     BearerAuth
// @Router       /seckill/activities/{activity_id}/buy [post]
func (h *SeckillHandler) Buy(c *gin.Context) {
	activityID64, err := strconv.ParseUint(c.Param("activity_id"), 10, 64)
	if err != nil || activityID64 == 0 {
		core.BadRequest(c, "秒杀活动 ID 不合法", err)
		return
	}
	userIDValue, ok := c.Get("userID")
	if !ok {
		core.Respond(c, http.StatusUnauthorized, core.MsgUnauthorized, nil, nil)
		return
	}
	userID, ok := userIDValue.(uint)
	if !ok || userID == 0 {
		core.Respond(c, http.StatusUnauthorized, core.MsgUnauthorized, nil, nil)
		return
	}
	if err := h.svc.Buy(uint(activityID64), userID); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "抢购请求已受理", gin.H{
		"status": "queued",
	})
}

// @Summary      秒杀活动列表
// @Description  返回进行中 / 未开始 / 最近结束的秒杀活动（公开接口，无需登录）。status 由时间窗口实时派生。
// @Tags         秒杀
// @Produce      json
// @Success      200  {object}  core.Response
// @Router       /seckill/activities [get]
func (h *SeckillHandler) ListActivities(c *gin.Context) {
	activities, err := h.svc.ListActivities()
	if err != nil {
		core.FailByError(c, err)
		return
	}
	items := make([]dto.SeckillActivityResponse, 0, len(activities))
	for _, activity := range activities {
		items = append(items, dto.SeckillActivityResponse{
			ID:                 activity.ID,
			ActivityNo:         activity.ActivityNo,
			Name:               activity.Name,
			ProductID:          activity.ProductID,
			ProductName:        activity.ProductName,
			ProductImage:       activity.ProductImage,
			OriginalPriceCents: activity.OriginalPriceCents,
			SeckillPriceCents:  activity.SeckillPriceCents,
			TotalStock:         activity.TotalStock,
			AvailableStock:     activity.AvailableStock,
			PerLimit:           activity.PerLimit,
			StartAt:            activity.StartAt,
			EndAt:              activity.EndAt,
			Status:             activity.Status,
		})
	}
	core.SuccessWithMessage(c, "获取秒杀活动列表成功", items)
}

// @Summary      查询我的抢购结果
// @Description  登录后查询当前用户在某活动是否抢到（前端轮询用）；未抢到返回 data=null
// @Tags         秒杀
// @Produce      json
// @Param        activity_id  path  uint  true  "秒杀活动ID"
// @Success      200  {object}  core.Response
// @Router       /seckill/activities/{activity_id}/my-result [get]
func (h *SeckillHandler) MyResult(c *gin.Context) {
	activityID64, err := strconv.ParseUint(c.Param("activity_id"), 10, 64)
	if err != nil || activityID64 == 0 {
		core.BadRequest(c, "秒杀活动 ID 不合法", err)
		return
	}
	userIDValue, ok := c.Get("userID")
	if !ok {
		core.Respond(c, http.StatusUnauthorized, core.MsgUnauthorized, nil, nil)
		return
	}
	userID, ok := userIDValue.(uint)
	if !ok || userID == 0 {
		core.Respond(c, http.StatusUnauthorized, core.MsgUnauthorized, nil, nil)
		return
	}
	p, err := h.svc.MyPurchase(userID, uint(activityID64))
	if err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "获取抢购结果成功", p)
}

// @Summary      初始化秒杀库存（管理员）
// @Description  将秒杀活动商品库存加载到 Redis
// @Tags         管理员
// @Accept       json
// @Produce      json
// @Param        body  body  dto.SeckillInitStockReq  true  "活动ID"
// @Success      200   {object}  core.Response
// @Failure      400   {object}  core.Response
// @Failure      401   {object}  core.Response
// @Failure      403   {object}  core.Response  "非管理员"
// @Security     BearerAuth
// @Router       /admin/seckill/activities/init-stock [post]
func (h *SeckillHandler) InitStock(c *gin.Context) {
	var req dto.SeckillInitStockReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.BadRequest(c, "请求参数不合法", err)
		return
	}
	if err := h.svc.InitActivityStock(req.ActivityID); err != nil {
		core.FailByError(c, err)
		return
	}
	core.SuccessWithMessage(c, "秒杀库存初始化成功", nil)
}
