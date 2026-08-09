# เวอร์ชันเครื่องมือปักไว้ให้ตรงกับ CI จะได้ไม่เจอ "เครื่องผมผ่านนะ"
GOLANGCI_VERSION := v2.12.2
SWAG_VERSION     := v1.16.4
AIR_VERSION      := latest

MIGRATIONS   := internal/migrate/migrations
TEST_DB_NAME ?= blog_user_api_test
GOBIN      := $(shell go env GOPATH)/bin

# อ่าน .env ถ้ามี — target ฝั่ง db ใช้ค่าพวกนี้ประกอบ DB_URL
ifneq (,$(wildcard .env))
include .env
export
endif

DB_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.DEFAULT_GOAL := help

## —— dev ——————————————————————————————————————————————

.PHONY: run
run: ## รัน API บนเครื่อง (ต้องมี postgres อยู่แล้ว)
	go run ./cmd/server

.PHONY: dev
dev: ## รันแบบ hot reload ด้วย air
	$(GOBIN)/air

.PHONY: build
build: ## build binary ลง bin/server
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server ./cmd/server

.PHONY: setup
setup: ## ก๊อป .env + โหลด dependency
	@test -f .env || cp .env.example .env
	go mod download

## —— คุณภาพ ——————————————————————————————————————————

.PHONY: fmt
fmt: ## จัดฟอร์แมตโค้ด
	gofmt -w .

.PHONY: vet
vet: ## go vet
	go vet ./...

.PHONY: test
test: ## รันเทสต์
	go test ./...

.PHONY: test-race
test-race: ## รันเทสต์พร้อม race detector (อันเดียวกับที่ CI รัน)
	go test -race ./...

.PHONY: test-integration
test-integration: ## เทสต์ repository กับ postgres จริง (ยก postgres ให้เอง)
	@docker compose up -d postgres >/dev/null
	@until docker compose exec -T postgres pg_isready -U $(DB_USER) >/dev/null 2>&1; do sleep 1; done
	@docker compose exec -T postgres psql -U $(DB_USER) -tAc \
		"SELECT 1 FROM pg_database WHERE datname='$(TEST_DB_NAME)'" | grep -q 1 \
		|| docker compose exec -T postgres createdb -U $(DB_USER) $(TEST_DB_NAME)
	TEST_DB_NAME=$(TEST_DB_NAME) go test -tags integration -count=1 ./internal/repository/...

.PHONY: cover
cover: ## ดู coverage เฉพาะ package ที่มีเทสต์
	go test -cover ./internal/auth/... ./internal/config/... ./internal/dto/request/... \
		./internal/handler/... ./internal/logging/... ./internal/middleware/... \
		./internal/model/... ./internal/response/... ./internal/service/... ./internal/validator/...

.PHONY: lint
lint: ## golangci-lint
	$(GOBIN)/golangci-lint run ./...

.PHONY: ci
ci: build vet test-race lint swag-check ## รันทุกอย่างที่ CI รัน ก่อน push ควรผ่านอันนี้ก่อน
	@echo "ผ่านหมด พร้อม push"

## —— docs ——————————————————————————————————————————————

.PHONY: swag
swag: ## gen swagger spec ใหม่จาก annotation
	$(GOBIN)/swag init -g cmd/server/main.go -o docs --parseInternal --parseDependency

.PHONY: swag-check
swag-check: ## เช็คว่า docs/ ยังตรงกับ annotation ในโค้ด
	@rm -rf .swagcheck
	@$(GOBIN)/swag init -g cmd/server/main.go -o .swagcheck --parseInternal --parseDependency >/dev/null
	@diff -q docs/swagger.json .swagcheck/swagger.json >/dev/null 2>&1 \
		&& diff -q docs/swagger.yaml .swagcheck/swagger.yaml >/dev/null 2>&1 \
		|| { rm -rf .swagcheck; echo "docs/ ไม่ตรงกับ annotation — สั่ง make swag แล้ว commit ด้วย"; exit 1; }
	@rm -rf .swagcheck

## —— docker ————————————————————————————————————————————

.PHONY: up
up: ## ยก postgres + api ขึ้นทั้งชุด
	docker compose up -d --build

.PHONY: down
down: ## หยุดทั้งชุด (ข้อมูลยังอยู่)
	docker compose down

.PHONY: clean
clean: ## หยุดทั้งชุด + ล้าง volume ของ DB
	docker compose down -v

.PHONY: logs
logs: ## ดู log ของ api
	docker compose logs -f api

.PHONY: ps
ps: ## ดูสถานะ container
	docker compose ps

## —— database ——————————————————————————————————————————

.PHONY: migrate-create
migrate-create: ## สร้าง migration ใหม่ — make migrate-create NAME=create_comments
	@test -n "$(NAME)" || { echo "ต้องใส่ NAME เช่น make migrate-create NAME=create_comments"; exit 1; }
	$(GOBIN)/migrate create -ext sql -dir $(MIGRATIONS) -seq $(NAME)

.PHONY: migrate-up
migrate-up: ## รัน migration (ปกติ app รันให้เองตอน start)
	$(GOBIN)/migrate -path $(MIGRATIONS) -database "$(DB_URL)" up

.PHONY: migrate-down
migrate-down: ## ถอย migration หนึ่งขั้น
	$(GOBIN)/migrate -path $(MIGRATIONS) -database "$(DB_URL)" down 1

.PHONY: migrate-version
migrate-version: ## ดูว่าตอนนี้อยู่ migration ไหน
	$(GOBIN)/migrate -path $(MIGRATIONS) -database "$(DB_URL)" version

.PHONY: psql
psql: ## เข้า psql ของ container
	docker compose exec postgres psql -U $(DB_USER) -d $(DB_NAME)

## —— smoke ——————————————————————————————————————————————

.PHONY: smoke
smoke: ## ยิง Postman collection ทั้งชุดด้วย newman (ต้อง up ไว้ก่อน)
	npx --yes newman@6 run postman/blog-user-api.postman_collection.json

## —— เครื่องมือ ————————————————————————————————————————

.PHONY: tools
tools: ## ลงเครื่องมือทั้งหมดตามเวอร์ชันที่ปักไว้
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/air-verse/air@$(AIR_VERSION)

.PHONY: help
help: ## ดูคำสั่งทั้งหมด
	@awk 'BEGIN {FS = ":.*?## "} \
		/^## —— / {gsub(/## —— /, ""); gsub(/ —+$$/, ""); printf "\n\033[1m%s\033[0m\n", $$0} \
		/^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
