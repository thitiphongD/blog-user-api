package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/thitiphongD/blog-user-api/internal/logging"
)

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := logging.WithRequestID(t.Context(), "abc123")

	if got := logging.RequestID(ctx); got != "abc123" {
		t.Fatalf("request id = %q", got)
	}
}

func TestRequestIDEmptyWhenMissing(t *testing.T) {
	if got := logging.RequestID(context.Background()); got != "" {
		t.Fatalf("request id = %q อยากได้ค่าว่าง", got)
	}
}

// logger ที่ได้จาก context ต้องแปะ request_id ให้เอง ไม่งั้นทุกจุดที่ log ต้องจำใส่เองทุกครั้ง
func TestFromContextAttachesRequestID(t *testing.T) {
	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(restore)

	ctx := logging.WithRequestID(t.Context(), "abc123")
	logging.FromContext(ctx).Info("ทดสอบ")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("อ่าน log ไม่ออก: %v (%s)", err, buf.String())
	}

	if line["request_id"] != "abc123" {
		t.Fatalf("log ไม่มี request_id: %s", buf.String())
	}
}

func TestFromContextWithoutRequestID(t *testing.T) {
	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(restore)

	logging.FromContext(context.Background()).Info("ทดสอบ")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("อ่าน log ไม่ออก: %v", err)
	}

	if _, ok := line["request_id"]; ok {
		t.Fatalf("ไม่มี request id แต่ดันแปะ key เปล่าๆ มา: %s", buf.String())
	}
}
