package repository

import (
	"context"
	"errors"

	"paysystem/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAccountNotFound  = errors.New("账户不存在")
	ErrBalanceNotEnough = errors.New("余额不足")
	ErrOptimisticLock   = errors.New("乐观锁冲突，请重试")
)

type AccountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) Create(ctx context.Context, account *model.Account) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *AccountRepository) GetByUserID(ctx context.Context, userID int64) (*model.Account, error) {
	var account model.Account
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	return &account, nil
}

func (r *AccountRepository) GetByUserIDForUpdate(ctx context.Context, tx *gorm.DB, userID int64) (*model.Account, error) {
	var account model.Account
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	return &account, nil
}

func (r *AccountRepository) Deduct(ctx context.Context, tx *gorm.DB, userID int64, amount int64, version int) error {
	result := tx.WithContext(ctx).
		Model(&model.Account{}).
		Where("user_id = ? AND balance >= ? AND version = ?", userID, amount, version).
		Updates(map[string]interface{}{
			"balance": gorm.Expr("balance - ?", amount),
			"version": gorm.Expr("version + 1"),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		account, err := r.GetByUserID(ctx, userID)
		if err != nil {
			return err
		}
		if account.Balance < amount {
			return ErrBalanceNotEnough
		}
		return ErrOptimisticLock
	}

	return nil
}

func (r *AccountRepository) Increase(ctx context.Context, tx *gorm.DB, userID int64, amount int64) error {
	result := tx.WithContext(ctx).
		Model(&model.Account{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"balance": gorm.Expr("balance + ?", amount),
			"version": gorm.Expr("version + 1"),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrAccountNotFound
	}

	return nil
}

func (r *AccountRepository) GetOrCreate(ctx context.Context, userID int64) (*model.Account, error) {
	account, err := r.GetByUserID(ctx, userID)
	if err == nil {
		return account, nil
	}

	if !errors.Is(err, ErrAccountNotFound) {
		return nil, err
	}

	newAccount := &model.Account{
		UserID:  userID,
		Balance: 0,
	}

	err = r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoNothing: true,
		}).
		Create(newAccount).Error

	if err != nil {
		return nil, err
	}

	return r.GetByUserID(ctx, userID)
}
