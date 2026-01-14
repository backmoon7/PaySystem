package service

import (
	"context"
	"time"

	"paysystem/internal/config"
	"paysystem/internal/model"
	"paysystem/internal/repository"
	"paysystem/pkg/idgen"

	"gorm.io/gorm"
)

type OrderService struct {
	orderRepo *repository.OrderRepository
	db        *gorm.DB
	cfg       *config.Config
}

func NewOrderService(db *gorm.DB, cfg *config.Config) *OrderService {
	return &OrderService{
		orderRepo: repository.NewOrderRepository(db),
		db:        db,
		cfg:       cfg,
	}
}

type CreateOrderRequest struct {
	RequestID   string
	UserID      int64
	Amount      int64
	ProductType string
	ProductID   string
}

func (s *OrderService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*model.PayOrder, error) {
	existingOrder, err := s.orderRepo.GetByRequestID(ctx, req.RequestID)
	if err != nil {
		return nil, err
	}
	if existingOrder != nil {
		return existingOrder, nil
	}

	orderNo := idgen.GenerateOrderNo()
	expiredAt := time.Now().Add(time.Duration(s.cfg.Business.OrderTimeoutMinutes) * time.Minute)

	order := &model.PayOrder{
		OrderNo:     orderNo,
		RequestID:   req.RequestID,
		UserID:      req.UserID,
		Amount:      req.Amount,
		ProductType: req.ProductType,
		ProductID:   req.ProductID,
		Status:      model.OrderStatusCreated,
		ExpiredAt:   expiredAt,
	}

	err = s.orderRepo.Create(ctx, nil, order)
	if err != nil {
		return nil, err
	}

	return order, nil
}

func (s *OrderService) GetOrder(ctx context.Context, orderNo string) (*model.PayOrder, error) {
	return s.orderRepo.GetByOrderNo(ctx, orderNo)
}

func (s *OrderService) GetOrderByRequestID(ctx context.Context, requestID string) (*model.PayOrder, error) {
	return s.orderRepo.GetByRequestID(ctx, requestID)
}

func (s *OrderService) CancelOrder(ctx context.Context, orderNo string) error {
	order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return err
	}

	return s.orderRepo.UpdateStatus(ctx, nil, orderNo, order.Status, model.OrderStatusCancelled)
}

func (s *OrderService) CloseExpiredOrders(ctx context.Context, limit int) (int, error) {
	orders, err := s.orderRepo.GetExpiredOrders(ctx, limit)
	if err != nil {
		return 0, err
	}

	closedCount := 0
	for _, order := range orders {
		err := s.orderRepo.UpdateStatus(ctx, nil, order.OrderNo, model.OrderStatusCreated, model.OrderStatusClosed)
		if err == nil {
			closedCount++
		}
	}

	return closedCount, nil
}

func (s *OrderService) ListUserOrders(ctx context.Context, userID int64, page, pageSize int) ([]*model.PayOrder, int64, error) {
	return s.orderRepo.ListByUserID(ctx, userID, page, pageSize)
}
