package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"paysystem/internal/config"
	"paysystem/internal/infrastructure/lock"
	"paysystem/internal/model"
	"paysystem/internal/repository"
	"paysystem/pkg/idgen"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type PayService struct {
	db              *gorm.DB
	redisClient     *redis.Client
	cfg             *config.Config
	orderRepo       *repository.OrderRepository
	accountRepo     *repository.AccountRepository
	transactionRepo *repository.TransactionRepository
	outboxRepo      *repository.OutboxRepository
}

func NewPayService(db *gorm.DB, redisClient *redis.Client, cfg *config.Config) *PayService {
	return &PayService{
		db:              db,
		redisClient:     redisClient,
		cfg:             cfg,
		orderRepo:       repository.NewOrderRepository(db),
		accountRepo:     repository.NewAccountRepository(db),
		transactionRepo: repository.NewTransactionRepository(db),
		outboxRepo:      repository.NewOutboxRepository(db),
	}
}

type PayRequest struct {
	RequestID   string `json:"request_id" binding:"required"`
	UserID      int64  `json:"user_id" binding:"required"`
	Amount      int64  `json:"amount" binding:"required,gt=0"`
	ProductType string `json:"product_type" binding:"required"`
	ProductID   string `json:"product_id" binding:"required"`
}

type PayResponse struct {
	OrderNo string `json:"order_no"`
	Status  string `json:"status"`
	Amount  int64  `json:"amount"`
	Message string `json:"message,omitempty"`
}

func (s *PayService) Pay(ctx context.Context, req *PayRequest) (*PayResponse, error) {
	// 幂等校验
	existingOrder, err := s.orderRepo.GetByRequestID(ctx, req.RequestID)
	if err != nil {
		return nil, fmt.Errorf("查询订单失败: %w", err)
	}

	if existingOrder != nil {
		return &PayResponse{
			OrderNo: existingOrder.OrderNo,
			Status:  existingOrder.Status,
			Amount:  existingOrder.Amount,
			Message: "订单已存在",
		}, nil
	}

	// 获取分布式锁
	payLock := lock.NewPayLock(s.redisClient, req.UserID, req.RequestID)
	err = payLock.Lock(ctx, 100*time.Millisecond, 30)
	if err != nil {
		return nil, fmt.Errorf("系统繁忙，请稍后重试: %w", err)
	}
	defer payLock.Unlock(ctx)

	// 获取锁后再次检查幂等
	existingOrder, err = s.orderRepo.GetByRequestID(ctx, req.RequestID)
	if err != nil {
		return nil, fmt.Errorf("查询订单失败: %w", err)
	}
	if existingOrder != nil {
		return &PayResponse{
			OrderNo: existingOrder.OrderNo,
			Status:  existingOrder.Status,
			Amount:  existingOrder.Amount,
			Message: "订单已存在",
		}, nil
	}

	// 检查账户余额
	account, err := s.accountRepo.GetOrCreate(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("获取账户信息失败: %w", err)
	}

	if account.Balance < req.Amount {
		return nil, errors.New("余额不足")
	}

	// 创建订单
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

	// 执行支付事务
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.orderRepo.Create(ctx, tx, order); err != nil {
			return fmt.Errorf("创建订单失败: %w", err)
		}

		if err := s.orderRepo.UpdateStatus(ctx, tx, orderNo, model.OrderStatusCreated, model.OrderStatusPaying); err != nil {
			return fmt.Errorf("更新订单状态失败: %w", err)
		}

		if err := s.accountRepo.Deduct(ctx, tx, req.UserID, req.Amount, account.Version); err != nil {
			if errors.Is(err, repository.ErrBalanceNotEnough) {
				return errors.New("余额不足")
			}
			if errors.Is(err, repository.ErrOptimisticLock) {
				return errors.New("系统繁忙，请重试")
			}
			return fmt.Errorf("扣款失败: %w", err)
		}

		transaction := &model.AccountTransaction{
			TransactionNo: idgen.GenerateTransactionNo(),
			UserID:        req.UserID,
			OrderNo:       orderNo,
			Amount:        -req.Amount,
			Type:          model.TransactionTypePay,
			BalanceBefore: account.Balance,
			BalanceAfter:  account.Balance - req.Amount,
			Remark:        fmt.Sprintf("支付-%s-%s", req.ProductType, req.ProductID),
		}
		if err := s.transactionRepo.Create(ctx, tx, transaction); err != nil {
			return fmt.Errorf("记录流水失败: %w", err)
		}

		now := time.Now()
		order.Status = model.OrderStatusPaid
		order.PaidAt = &now
		if err := s.orderRepo.UpdateStatus(ctx, tx, orderNo, model.OrderStatusPaying, model.OrderStatusPaid); err != nil {
			return fmt.Errorf("更新订单状态失败: %w", err)
		}

		msgPayload := map[string]interface{}{
			"order_no":     orderNo,
			"user_id":      req.UserID,
			"amount":       req.Amount,
			"product_type": req.ProductType,
			"product_id":   req.ProductID,
			"status":       model.OrderStatusPaid,
			"paid_at":      now.Format(time.RFC3339),
		}
		payloadBytes, _ := json.Marshal(msgPayload)

		outboxMsg := &model.OutboxMessage{
			MessageKey: orderNo,
			Topic:      s.cfg.Kafka.Topic.PayResult,
			Payload:    string(payloadBytes),
			Status:     model.OutboxStatusPending,
		}
		if err := s.outboxRepo.Create(ctx, tx, outboxMsg); err != nil {
			return fmt.Errorf("写入消息失败: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	log.Printf("支付成功: orderNo=%s, userID=%d, amount=%d", orderNo, req.UserID, req.Amount)

	return &PayResponse{
		OrderNo: orderNo,
		Status:  model.OrderStatusPaid,
		Amount:  req.Amount,
		Message: "支付成功",
	}, nil
}

func (s *PayService) QueryPayResult(ctx context.Context, orderNo string) (*PayResponse, error) {
	order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, errors.New("订单不存在")
		}
		return nil, err
	}

	return &PayResponse{
		OrderNo: order.OrderNo,
		Status:  order.Status,
		Amount:  order.Amount,
	}, nil
}

func (s *PayService) QueryPayResultByRequestID(ctx context.Context, requestID string) (*PayResponse, error) {
	order, err := s.orderRepo.GetByRequestID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.New("订单不存在")
	}

	return &PayResponse{
		OrderNo: order.OrderNo,
		Status:  order.Status,
		Amount:  order.Amount,
	}, nil
}
