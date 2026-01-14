package repository

import (
	"context"
	"errors"
	"time"

	"paysystem/internal/model"

	"gorm.io/gorm"
)

var (
	ErrOrderNotFound      = errors.New("订单不存在")
	ErrOrderStatusInvalid = errors.New("订单状态不合法")
	ErrDuplicateRequest   = errors.New("重复请求")
)

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(ctx context.Context, tx *gorm.DB, order *model.PayOrder) error {
	if tx == nil {
		tx = r.db
	}
	return tx.WithContext(ctx).Create(order).Error
}

func (r *OrderRepository) GetByOrderNo(ctx context.Context, orderNo string) (*model.PayOrder, error) {
	var order model.PayOrder
	err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) GetByRequestID(ctx context.Context, requestID string) (*model.PayOrder, error) {
	var order model.PayOrder
	err := r.db.WithContext(ctx).Where("request_id = ?", requestID).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, orderNo string, fromStatus, toStatus string) error {
	if !model.CanTransitionTo(fromStatus, toStatus) {
		return ErrOrderStatusInvalid
	}

	if tx == nil {
		tx = r.db
	}

	updates := map[string]interface{}{
		"status": toStatus,
	}

	if toStatus == model.OrderStatusPaid {
		now := time.Now()
		updates["paid_at"] = &now
	}

	result := tx.WithContext(ctx).
		Model(&model.PayOrder{}).
		Where("order_no = ? AND status = ?", orderNo, fromStatus).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrOrderStatusInvalid
	}

	return nil
}

func (r *OrderRepository) GetExpiredOrders(ctx context.Context, limit int) ([]*model.PayOrder, error) {
	var orders []*model.PayOrder
	err := r.db.WithContext(ctx).
		Where("status = ? AND expired_at < ?", model.OrderStatusCreated, time.Now()).
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

func (r *OrderRepository) GetPayingOrders(ctx context.Context, beforeTime time.Time, limit int) ([]*model.PayOrder, error) {
	var orders []*model.PayOrder
	err := r.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", model.OrderStatusPaying, beforeTime).
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

func (r *OrderRepository) ListByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*model.PayOrder, int64, error) {
	var orders []*model.PayOrder
	var total int64

	query := r.db.WithContext(ctx).Model(&model.PayOrder{}).Where("user_id = ?", userID)

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&orders).Error

	return orders, total, err
}
