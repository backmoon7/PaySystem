package handler

import (
	"strconv"

	"paysystem/internal/config"
	"paysystem/internal/service"
	"paysystem/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Handler 统一处理器，包含所有服务依赖
type Handler struct {
	accountService *service.AccountService
	orderService   *service.OrderService
	payService     *service.PayService
	refundService  *service.RefundService
}

// NewHandler 创建处理器实例
func NewHandler(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *Handler {
	return &Handler{
		accountService: service.NewAccountService(db),
		orderService:   service.NewOrderService(db, cfg),
		payService:     service.NewPayService(db, rdb, cfg),
		refundService:  service.NewRefundService(db, rdb, cfg),
	}
}

// ============================================================
// 账户相关接口
// ============================================================

// GetBalance 查询用户余额
// GET /api/v1/account/balance?user_id=xxx
func (h *Handler) GetBalance(c *gin.Context) {
	userIDStr := c.Query("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.ParamError(c, "user_id 参数错误")
		return
	}

	account, err := h.accountService.GetAccount(c.Request.Context(), userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"user_id":       account.UserID,
		"balance":       account.Balance,
		"frozen_amount": account.FrozenAmount,
	})
}

// RechargeRequest 充值请求
type RechargeRequest struct {
	UserID int64 `json:"user_id" binding:"required"`
	Amount int64 `json:"amount" binding:"required,gt=0"`
}

// Recharge 充值接口（简化版，实际应该走支付渠道）
// POST /api/v1/account/recharge
func (h *Handler) Recharge(c *gin.Context) {
	var req RechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "参数错误: "+err.Error())
		return
	}

	if err := h.accountService.Recharge(c.Request.Context(), req.UserID, req.Amount); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"message": "充值成功",
	})
}

// ============================================================
// 订单相关接口
// ============================================================

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	RequestID   string `json:"request_id" binding:"required"` // 幂等ID
	UserID      int64  `json:"user_id" binding:"required"`
	Amount      int64  `json:"amount" binding:"required,gt=0"`
	ProductType string `json:"product_type" binding:"required"` // 如 video, article
	ProductID   string `json:"product_id" binding:"required"`   // 投币目标ID
}

// CreateOrder 创建订单
// POST /api/v1/order/create
func (h *Handler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "参数错误: "+err.Error())
		return
	}

	serviceReq := &service.CreateOrderRequest{
		RequestID:   req.RequestID,
		UserID:      req.UserID,
		Amount:      req.Amount,
		ProductType: req.ProductType,
		ProductID:   req.ProductID,
	}

	order, err := h.orderService.CreateOrder(c.Request.Context(), serviceReq)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"order_no": order.OrderNo,
		"status":   order.Status,
		"amount":   order.Amount,
	})
}

// GetOrder 查询订单详情
// GET /api/v1/order/detail?order_no=xxx
func (h *Handler) GetOrder(c *gin.Context) {
	orderNo := c.Query("order_no")
	if orderNo == "" {
		response.ParamError(c, "order_no 参数不能为空")
		return
	}

	order, err := h.orderService.GetOrder(c.Request.Context(), orderNo)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, order)
}

// ListOrders 查询用户订单列表
// GET /api/v1/order/list?user_id=xxx&page=1&page_size=10
func (h *Handler) ListOrders(c *gin.Context) {
	userIDStr := c.Query("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.ParamError(c, "user_id 参数错误")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	orders, total, err := h.orderService.ListUserOrders(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"list":      orders,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CancelOrder 取消订单
// POST /api/v1/order/cancel
func (h *Handler) CancelOrder(c *gin.Context) {
	var req struct {
		OrderNo string `json:"order_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "参数错误: "+err.Error())
		return
	}

	if err := h.orderService.CancelOrder(c.Request.Context(), req.OrderNo); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"message": "订单已取消",
	})
}

// ============================================================
// 支付相关接口
// ============================================================

// PayOrderRequest 支付请求
type PayOrderRequest struct {
	RequestID   string `json:"request_id" binding:"required"`   // 幂等性ID，客户端生成
	UserID      int64  `json:"user_id" binding:"required"`      // 用户ID
	Amount      int64  `json:"amount" binding:"required,gt=0"`  // 支付金额
	ProductType string `json:"product_type" binding:"required"` // 产品类型
	ProductID   string `json:"product_id" binding:"required"`   // 产品ID
}

// PayOrder 支付订单
// POST /api/v1/pay/execute
//
// 【关键点】支付是整个系统最核心的操作，需要保证：
// 1. 幂等性：相同的 request_id 只会执行一次
// 2. 原子性：订单状态更新、余额扣减、流水记录必须同时成功或同时失败
// 3. 并发安全：通过分布式锁防止重复支付
func (h *Handler) PayOrder(c *gin.Context) {
	var req PayOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "参数错误: "+err.Error())
		return
	}

	payReq := &service.PayRequest{
		RequestID:   req.RequestID,
		UserID:      req.UserID,
		Amount:      req.Amount,
		ProductType: req.ProductType,
		ProductID:   req.ProductID,
	}

	result, err := h.payService.Pay(c.Request.Context(), payReq)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, result)
}

// ============================================================
// 退款相关接口
// ============================================================

// RefundOrderRequest 退款请求
type RefundOrderRequest struct {
	OrderNo   string `json:"order_no" binding:"required"`
	RequestID string `json:"request_id" binding:"required"` // 幂等性ID
	Reason    string `json:"reason"`
}

// RefundOrder 退款
// POST /api/v1/refund/execute
//
// 【关键点】退款流程：
// 1. 只支持全额退款
// 2. 订单状态必须是 PAID 才能退款
// 3. 退款成功后恢复用户余额
func (h *Handler) RefundOrder(c *gin.Context) {
	var req RefundOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamError(c, "参数错误: "+err.Error())
		return
	}

	refundReq := &service.RefundRequest{
		OrderNo:   req.OrderNo,
		RequestID: req.RequestID,
		Reason:    req.Reason,
	}

	result, err := h.refundService.Refund(c.Request.Context(), refundReq)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.Success(c, result)
}
