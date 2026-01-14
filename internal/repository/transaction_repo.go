package repository

import (
	"context"

	"paysystem/internal/model"

	"gorm.io/gorm"
)

type TransactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Create(ctx context.Context, tx *gorm.DB, trans *model.AccountTransaction) error {
	if tx == nil {
		tx = r.db
	}
	return tx.WithContext(ctx).Create(trans).Error
}

func (r *TransactionRepository) GetByOrderNo(ctx context.Context, orderNo string) (*model.AccountTransaction, error) {
	var trans model.AccountTransaction
	err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&trans).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &trans, nil
}

func (r *TransactionRepository) GetByTransactionNo(ctx context.Context, transactionNo string) (*model.AccountTransaction, error) {
	var trans model.AccountTransaction
	err := r.db.WithContext(ctx).Where("transaction_no = ?", transactionNo).First(&trans).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &trans, nil
}

func (r *TransactionRepository) ListByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*model.AccountTransaction, int64, error) {
	var transactions []*model.AccountTransaction
	var total int64

	query := r.db.WithContext(ctx).Model(&model.AccountTransaction{}).Where("user_id = ?", userID)

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&transactions).Error

	return transactions, total, err
}

func (r *TransactionRepository) GetByUserIDAndOrderNo(ctx context.Context, userID int64, orderNo string) (*model.AccountTransaction, error) {
	var trans model.AccountTransaction
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND order_no = ?", userID, orderNo).
		First(&trans).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &trans, nil
}
