FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 บังคับ เพราะ runtime image เป็น alpine (musl) ถ้าไม่ปิดจะได้ binary
# ที่ link glibc แล้วรันไม่ขึ้น ขึ้น "no such file or directory" ทั้งที่ไฟล์อยู่ตรงนั้น
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20

RUN adduser -D -u 10001 app

WORKDIR /app
COPY --from=builder /out/server /app/server

USER app
EXPOSE 8080

ENTRYPOINT ["/app/server"]
