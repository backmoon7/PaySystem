package repository

import (
	"context"

	"paysystem/internal/model"

	"gorm.io/gorm"
)

type OutboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Create(ctx context.Context, tx *gorm.DB, msg *model.OutboxMessage) error {
	if tx == nil {
		tx = r.db
	}
	return tx.WithContext(ctx).Create(msg).Error
}

func (r *OutboxRepository) GetPendingMessages(ctx context.Context, limit int) ([]*model.OutboxMessage, error) {
	var messages []*model.OutboxMessage
	err := r.db.WithContext(ctx).
		Where("status = ?", model.OutboxStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

func (r *OutboxRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.OutboxMessage{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *OutboxRepository) IncrementRetryCount(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.OutboxMessage{}).
		Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

func (r *OutboxRepository) MarkAsFailed(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.OutboxMessage{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      model.OutboxStatusFailed,
			"retry_count": gorm.Expr("retry_count + 1"),
		}).Error
}

func (r *OutboxRepository) GetFailedMessages(ctx context.Context, limit int) ([]*model.OutboxMessage, error) {
	var messages []*model.OutboxMessage
	err := r.db.WithContext(ctx).
		Where("status = ?", model.OutboxStatusFailed).
		Order("created_at ASC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}
