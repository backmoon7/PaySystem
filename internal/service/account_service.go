package service

import (
	"context"
	"errors"

	"paysystem/internal/model"
	"paysystem/internal/repository"

	"gorm.io/gorm"
)

type AccountService struct {
	accountRepo *repository.AccountRepository
	db          *gorm.DB
}

func NewAccountService(db *gorm.DB) *AccountService {
	return &AccountService{
		accountRepo: repository.NewAccountRepository(db),
		db:          db,
	}
}

func (s *AccountService) GetBalance(ctx context.Context, userID int64) (int64, error) {
	account, err := s.accountRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrAccountNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return account.Balance, nil
}

func (s *AccountService) GetAccount(ctx context.Context, userID int64) (*model.Account, error) {
	return s.accountRepo.GetOrCreate(ctx, userID)
}

func (s *AccountService) Recharge(ctx context.Context, userID int64, amount int64) error {
	if amount <= 0 {
		return errors.New("充值金额必须大于0")
	}

	_, err := s.accountRepo.GetOrCreate(ctx, userID)
	if err != nil {
		return err
	}

	return s.accountRepo.Increase(ctx, s.db, userID, amount)
}
