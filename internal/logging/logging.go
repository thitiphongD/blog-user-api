// Package logging พา request_id ไปกับ context.Context เพื่อให้ layer ที่ไม่รู้จัก echo
// (service, repository) log แล้วมี request_id ติดไปด้วย
package logging

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// FromContext คืน logger ที่ติด request_id มาแล้ว ถ้า context ไม่มีก็คืน logger เปล่า
func FromContext(ctx context.Context) *slog.Logger {
	if id := RequestID(ctx); id != "" {
		return slog.Default().With("request_id", id)
	}
	return slog.Default()
}
