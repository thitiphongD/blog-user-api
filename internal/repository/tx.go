package repository

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// TxManager รัน fn ใน transaction เดียว โดยส่ง tx ไปกับ context
// repository ทุกตัวเรียก conn(ctx) เลยหยิบ tx ตัวเดียวกันได้เองโดย service
// ไม่ต้องรู้ว่ามี *gorm.DB อยู่ตรงไหน
type TxManager struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db: db}
}

func (t *TxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// conn คืน tx ถ้ากำลังอยู่ใน transaction ไม่งั้นคืน connection ปกติ
func conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return db.WithContext(ctx)
}
