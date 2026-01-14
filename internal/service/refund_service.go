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

type RefundService struct {
	db              *gorm.DB
	redisClient     *redis.Client
	cfg             *config.Config
	orderRepo       *repository.OrderRepository
	accountRepo     *repository.AccountRepository
	transactionRepo *repository.TransactionRepository
	outboxRepo      *repository.OutboxRepository
}

func NewRefundService(db *gorm.DB, redisClient *redis.Client, cfg *config.Config) *RefundService {
	return &RefundService{
		db:              db,
		redisClient:     redisClient,
		cfg:             cfg,
		orderRepo:       repository.NewOrderRepository(db),
		accountRepo:     repository.NewAccountRepository(db),
		transactionRepo: repository.NewTransactionRepository(db),
		outboxRepo:      repository.NewOutboxRepository(db),
	}
}

type RefundRequest struct {
	RequestID string `json:"request_id" binding:"required"`
	OrderNo   string `json:"order_no" binding:"required"`
	Reason    string `json:"reason"`
}

type RefundResponse struct {
	RefundNo string `json:"refund_no"`
	OrderNo  string `json:"order_no"`
	Amount   int64  `json:"amount"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

func (s *RefundService) Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	order, err := s.orderRepo.GetByOrderNo(ctx, req.OrderNo)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, errors.New("订单不存在")
		}
		return nil, fmt.Errorf("查询订单失败: %w", err)
	}

	if order.Status != model.OrderStatusPaid {
		return nil, fmt.Errorf("订单状态不允许退款，当前状态: %s", order.Status)
	}

	existingTrans, err := s.transactionRepo.GetByUserIDAndOrderNo(ctx, order.UserID, req.OrderNo)
	if err != nil {
		return nil, fmt.Errorf("查询流水失败: %w", err)
	}
	if existingTrans != nil && existingTrans.Type == model.TransactionTypeRefund {
		return &RefundResponse{
			OrderNo: order.OrderNo,
			Amount:  order.Amount,
			Status:  model.OrderStatusRefunded,
			Message: "已退款，请勿重复操作",
		}, nil
	}

	refundLock := lock.NewDistributedLock(
		s.redisClient,
		fmt.Sprintf("refund:lock:order:%s", req.OrderNo),
		req.RequestID,
		30*time.Second,
	)
	err = refundLock.Lock(ctx, 100*time.Millisecond, 30)
	if err != nil {
		return nil, fmt.Errorf("系统繁忙，请稍后重试: %w", err)
	}
	defer refundLock.Unlock(ctx)

	order, err = s.orderRepo.GetByOrderNo(ctx, req.OrderNo)
	if err != nil {
		return nil, err
	}
	if order.Status != model.OrderStatusPaid {
		if order.Status == model.OrderStatusRefunded {
			return &RefundResponse{
				OrderNo: order.OrderNo,
				Amount:  order.Amount,
				Status:  model.OrderStatusRefunded,
				Message: "已退款，请勿重复操作",
			}, nil
		}
		return nil, fmt.Errorf("订单状态不允许退款，当前状态: %s", order.Status)
	}

	refundNo := idgen.GenerateRefundNo()

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.orderRepo.UpdateStatus(ctx, tx, req.OrderNo, model.OrderStatusPaid, model.OrderStatusRefunding); err != nil {
			return fmt.Errorf("更新订单状态失败: %w", err)
		}

		account, err := s.accountRepo.GetByUserID(ctx, order.UserID)
		if err != nil {
			return fmt.Errorf("查询账户失败: %w", err)
		}

		if err := s.accountRepo.Increase(ctx, tx, order.UserID, order.Amount); err != nil {
			return fmt.Errorf("退款到账失败: %w", err)
		}

		transaction := &model.AccountTransaction{
			TransactionNo: idgen.GenerateTransactionNo(),
			UserID:        order.UserID,
			OrderNo:       req.OrderNo,
			Amount:        order.Amount,
			Type:          model.TransactionTypeRefund,
			BalanceBefore: account.Balance,
			BalanceAfter:  account.Balance + order.Amount,
			Remark:        fmt.Sprintf("退款-%s-%s", refundNo, req.Reason),
		}
		if err := s.transactionRepo.Create(ctx, tx, transaction); err != nil {
			return fmt.Errorf("记录流水失败: %w", err)
		}

		if err := s.orderRepo.UpdateStatus(ctx, tx, req.OrderNo, model.OrderStatusRefunding, model.OrderStatusRefunded); err != nil {
			return fmt.Errorf("更新订单状态失败: %w", err)
		}

		msgPayload := map[string]interface{}{
			"refund_no":   refundNo,
			"order_no":    req.OrderNo,
			"user_id":     order.UserID,
			"amount":      order.Amount,
			"status":      model.OrderStatusRefunded,
			"reason":      req.Reason,
			"refunded_at": time.Now().Format(time.RFC3339),
		}
		payloadBytes, _ := json.Marshal(msgPayload)

		outboxMsg := &model.OutboxMessage{
			MessageKey: refundNo,
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

	log.Printf("退款成功: refundNo=%s, orderNo=%s, amount=%d", refundNo, req.OrderNo, order.Amount)

	return &RefundResponse{
		RefundNo: refundNo,
		OrderNo:  req.OrderNo,
		Amount:   order.Amount,
		Status:   model.OrderStatusRefunded,
		Message:  "退款成功",
	}, nil
}
