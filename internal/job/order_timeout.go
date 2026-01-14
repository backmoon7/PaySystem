package job

import (
	"context"
	"log"
	"time"

	"paysystem/internal/config"
	"paysystem/internal/model"
	"paysystem/internal/repository"

	"gorm.io/gorm"
)

type OrderTimeoutJob struct {
	db        *gorm.DB
	orderRepo *repository.OrderRepository
	cfg       *config.Config
	stopCh    chan struct{}
	interval  time.Duration
	batchSize int
}

func NewOrderTimeoutJob(db *gorm.DB, cfg *config.Config) *OrderTimeoutJob {
	return &OrderTimeoutJob{
		db:        db,
		orderRepo: repository.NewOrderRepository(db),
		cfg:       cfg,
		stopCh:    make(chan struct{}),
		interval:  10 * time.Second,
		batchSize: 100,
	}
}

func (j *OrderTimeoutJob) Start(ctx context.Context) {
	log.Println("[OrderTimeoutJob] 订单超时任务启动")

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[OrderTimeoutJob] 收到停止信号，任务退出")
			return
		case <-j.stopCh:
			log.Println("[OrderTimeoutJob] 任务停止")
			return
		case <-ticker.C:
			j.closeExpiredOrders(ctx)
		}
	}
}

func (j *OrderTimeoutJob) Stop() {
	close(j.stopCh)
}

func (j *OrderTimeoutJob) closeExpiredOrders(ctx context.Context) {
	orders, err := j.orderRepo.GetExpiredOrders(ctx, j.batchSize)
	if err != nil {
		log.Printf("[OrderTimeoutJob] 查询超时订单失败: %v", err)
		return
	}

	if len(orders) == 0 {
		return
	}

	log.Printf("[OrderTimeoutJob] 发现 %d 个超时订单", len(orders))

	closedCount := 0
	for _, order := range orders {
		err := j.orderRepo.UpdateStatus(ctx, nil, order.OrderNo, model.OrderStatusCreated, model.OrderStatusClosed)
		if err != nil {
			log.Printf("[OrderTimeoutJob] 关闭订单失败: orderNo=%s, err=%v", order.OrderNo, err)
			continue
		}
		closedCount++
		log.Printf("[OrderTimeoutJob] 订单已超时关闭: orderNo=%s, userID=%d, amount=%d",
			order.OrderNo, order.UserID, order.Amount)
	}

	log.Printf("[OrderTimeoutJob] 本次关闭 %d 个超时订单", closedCount)
}

type PayingOrderCompensateJob struct {
	db              *gorm.DB
	orderRepo       *repository.OrderRepository
	transactionRepo *repository.TransactionRepository
	cfg             *config.Config
	stopCh          chan struct{}
	interval        time.Duration
	batchSize       int
}

func NewPayingOrderCompensateJob(db *gorm.DB, cfg *config.Config) *PayingOrderCompensateJob {
	return &PayingOrderCompensateJob{
		db:              db,
		orderRepo:       repository.NewOrderRepository(db),
		transactionRepo: repository.NewTransactionRepository(db),
		cfg:             cfg,
		stopCh:          make(chan struct{}),
		interval:        30 * time.Second,
		batchSize:       50,
	}
}

func (j *PayingOrderCompensateJob) Start(ctx context.Context) {
	log.Println("[PayingOrderCompensateJob] 补偿任务启动")

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[PayingOrderCompensateJob] 收到停止信号，任务退出")
			return
		case <-j.stopCh:
			log.Println("[PayingOrderCompensateJob] 任务停止")
			return
		case <-ticker.C:
			j.compensatePayingOrders(ctx)
		}
	}
}

func (j *PayingOrderCompensateJob) Stop() {
	close(j.stopCh)
}

func (j *PayingOrderCompensateJob) compensatePayingOrders(ctx context.Context) {
	beforeTime := time.Now().Add(-5 * time.Minute)
	orders, err := j.orderRepo.GetPayingOrders(ctx, beforeTime, j.batchSize)
	if err != nil {
		log.Printf("[PayingOrderCompensateJob] 查询订单失败: %v", err)
		return
	}

	if len(orders) == 0 {
		return
	}

	log.Printf("[PayingOrderCompensateJob] 发现 %d 个需要补偿的订单", len(orders))

	for _, order := range orders {
		j.compensateOrder(ctx, order)
	}
}

func (j *PayingOrderCompensateJob) compensateOrder(ctx context.Context, order *model.PayOrder) {
	trans, err := j.transactionRepo.GetByOrderNo(ctx, order.OrderNo)
	if err != nil {
		log.Printf("[PayingOrderCompensateJob] 查询流水失败: orderNo=%s, err=%v", order.OrderNo, err)
		return
	}

	if trans != nil && trans.Type == model.TransactionTypePay {
		log.Printf("[PayingOrderCompensateJob] 发现已扣款但状态未更新的订单: orderNo=%s", order.OrderNo)

		err := j.orderRepo.UpdateStatus(ctx, nil, order.OrderNo, model.OrderStatusPaying, model.OrderStatusPaid)
		if err != nil {
			log.Printf("[PayingOrderCompensateJob] 补偿更新订单状态失败: orderNo=%s, err=%v", order.OrderNo, err)
		} else {
			log.Printf("[PayingOrderCompensateJob] 补偿成功，订单状态已更新为 PAID: orderNo=%s", order.OrderNo)
		}
		return
	}

	orderTimeout := time.Duration(j.cfg.Business.OrderTimeoutMinutes) * time.Minute
	if time.Since(order.CreatedAt) > orderTimeout {
		log.Printf("[PayingOrderCompensateJob] 订单超时且无扣款流水，准备关闭: orderNo=%s", order.OrderNo)

		err := j.orderRepo.UpdateStatus(ctx, nil, order.OrderNo, model.OrderStatusPaying, model.OrderStatusFailed)
		if err != nil {
			log.Printf("[PayingOrderCompensateJob] 关闭订单失败: orderNo=%s, err=%v", order.OrderNo, err)
		} else {
			log.Printf("[PayingOrderCompensateJob] 订单已标记为失败: orderNo=%s", order.OrderNo)
		}
	}
}
