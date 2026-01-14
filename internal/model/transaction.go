package model

import (
	"time"
)

// ============================================================================
// 交易类型常量
// ============================================================================

const (
	TransactionTypeRecharge = "RECHARGE" // 充值
	TransactionTypePay      = "PAY"      // 支付（扣款）
	TransactionTypeRefund   = "REFUND"   // 退款
)

// ============================================================================
// 账户流水实体
// ============================================================================

// AccountTransaction 账户流水表
// 记录账户的每一笔资金变动，是对账的核心依据
//
// 【重要】流水表设计原则：
// 1. 只追加，不修改，不删除 —— 保证审计可追溯
// 2. 每笔流水必须关联订单号 —— 便于对账
// 3. 记录交易前后余额 —— 便于校验余额一致性
type AccountTransaction struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TransactionNo string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"transaction_no"` // 流水号（全局唯一）
	UserID        int64     `gorm:"index;not null" json:"user_id"`                               // 用户ID
	OrderNo       string    `gorm:"type:varchar(64);index;not null" json:"order_no"`             // 关联订单号
	Amount        int64     `gorm:"not null" json:"amount"`                                      // 金额（正数入账，负数出账）
	Type          string    `gorm:"type:varchar(20);not null" json:"type"`                       // 交易类型
	BalanceBefore int64     `gorm:"not null" json:"balance_before"`                              // 交易前余额
	BalanceAfter  int64     `gorm:"not null" json:"balance_after"`                               // 交易后余额
	Remark        string    `gorm:"type:varchar(256)" json:"remark"`                             // 备注
	CreatedAt     time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (AccountTransaction) TableName() string {
	return "account_transaction"
}
