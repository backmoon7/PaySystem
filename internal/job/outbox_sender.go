package job

import (
	"context"
	"log"
	"time"

	"paysystem/internal/config"
	"paysystem/internal/infrastructure/mq"
	"paysystem/internal/model"
	"paysystem/internal/repository"

	"gorm.io/gorm"
)

type OutboxSender struct {
	db         *gorm.DB
	outboxRepo *repository.OutboxRepository
	cfg        *config.Config
	stopCh     chan struct{}
	interval   time.Duration
	batchSize  int
}

func NewOutboxSender(db *gorm.DB, cfg *config.Config) *OutboxSender {
	return &OutboxSender{
		db:         db,
		outboxRepo: repository.NewOutboxRepository(db),
		cfg:        cfg,
		stopCh:     make(chan struct{}),
		interval:   100 * time.Millisecond,
		batchSize:  100,
	}
}

func (s *OutboxSender) Start(ctx context.Context) {
	log.Println("[OutboxSender] 消息发送任务启动")

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[OutboxSender] 收到停止信号，任务退出")
			return
		case <-s.stopCh:
			log.Println("[OutboxSender] 任务停止")
			return
		case <-ticker.C:
			s.processPendingMessages(ctx)
		}
	}
}

func (s *OutboxSender) Stop() {
	close(s.stopCh)
}

func (s *OutboxSender) processPendingMessages(ctx context.Context) {
	messages, err := s.outboxRepo.GetPendingMessages(ctx, s.batchSize)
	if err != nil {
		log.Printf("[OutboxSender] 查询消息失败: %v", err)
		return
	}

	if len(messages) == 0 {
		return
	}

	for _, msg := range messages {
		s.sendMessage(ctx, msg)
	}
}

func (s *OutboxSender) sendMessage(ctx context.Context, msg *model.OutboxMessage) {
	err := mq.SendMessage(msg.Topic, msg.MessageKey, msg.Payload)

	if err == nil {
		if updateErr := s.outboxRepo.UpdateStatus(ctx, msg.ID, model.OutboxStatusSent); updateErr != nil {
			log.Printf("[OutboxSender] 更新消息状态失败: id=%d, err=%v", msg.ID, updateErr)
		} else {
			log.Printf("[OutboxSender] 消息发送成功: id=%d, topic=%s, key=%s", msg.ID, msg.Topic, msg.MessageKey)
		}
		return
	}

	log.Printf("[OutboxSender] 消息发送失败: id=%d, err=%v", msg.ID, err)

	if err := s.outboxRepo.IncrementRetryCount(ctx, msg.ID); err != nil {
		log.Printf("[OutboxSender] 增加重试次数失败: id=%d, err=%v", msg.ID, err)
	}

	if msg.RetryCount+1 >= s.cfg.Business.MaxRetryCount {
		if err := s.outboxRepo.MarkAsFailed(ctx, msg.ID); err != nil {
			log.Printf("[OutboxSender] 标记消息失败状态失败: id=%d, err=%v", msg.ID, err)
		} else {
			log.Printf("[OutboxSender] 消息超过最大重试次数，标记为失败: id=%d", msg.ID)
		}
	}
}
