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
migrate-create: ## สร้าง migration เปล่าไว้เขียน SQL เอง (ปกติใช้ make prisma-diff แทน)
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

## —— prisma ————————————————————————————————————————————

# Prisma ทำหน้าที่เดียวคือแปลง prisma/schema.prisma เป็น SQL
# คนรัน migration ยังเป็น golang-migrate เหมือนเดิม ไม่มี prisma client ในโปรเจกต์นี้
# และไม่มี Node ใน image ที่ deploy จริง ($(MIGRATIONS)/*.sql ฝังเข้า binary ตอน build)
PRISMA := npx prisma

# ลง prisma ให้เองครั้งแรก แทนที่จะขึ้น error ว่าหาไม่เจอ — เวอร์ชันปักไว้ที่ package.json
node_modules: package.json package-lock.json
	npm install
	@touch node_modules

.PHONY: prisma-init
prisma-init: node_modules ## gen migration ก้อนแรกใหม่ทั้งหมด — ล้าง $(MIGRATIONS) ทิ้ง ใช้ตอนตั้งต้นเท่านั้น
	@tmp=$$(mktemp); \
	rm -f $(MIGRATIONS)/*.sql; \
	DATABASE_URL="$(DB_URL)" $(PRISMA) migrate diff --from-empty \
		--to-schema prisma/schema.prisma --script -o $$tmp >/dev/null; \
	cat $$tmp prisma/extras.sql > $(MIGRATIONS)/000001_init.up.sql; \
	rm -f $$tmp; \
	DATABASE_URL="$(DB_URL)" $(PRISMA) migrate diff --from-schema prisma/schema.prisma \
		--to-empty --script -o $(MIGRATIONS)/000001_init.down.sql >/dev/null
	@echo "gen แล้ว: $(MIGRATIONS)/000001_init.{up,down}.sql"
	@echo "DB ที่มีข้อมูลอยู่แล้วใช้ต่อไม่ได้ ต้อง make clean แล้วขึ้นใหม่"

.PHONY: prisma-diff
prisma-diff: node_modules ## gen migration ใบถัดไปจากส่วนต่าง DB จริง vs schema.prisma — make prisma-diff NAME=add_tags
	@test -n "$(NAME)" || { echo "ต้องใส่ NAME เช่น make prisma-diff NAME=add_tags"; exit 1; }
	@tmp=$$(mktemp); \
	last=$$(ls $(MIGRATIONS)/*.up.sql 2>/dev/null | sed 's|.*/||; s/_.*//' | sort -n | tail -1); \
	next=$$(printf '%06d' $$((10#$${last:-0} + 1))); \
	DATABASE_URL="$(DB_URL)" $(PRISMA) migrate diff --from-config-datasource \
		--to-schema prisma/schema.prisma --script -o $$tmp >/dev/null; \
	cat $$tmp prisma/extras.sql > $(MIGRATIONS)/$${next}_$(NAME).up.sql; \
	rm -f $$tmp; \
	DATABASE_URL="$(DB_URL)" $(PRISMA) migrate diff --from-schema prisma/schema.prisma \
		--to-config-datasource --script -o $(MIGRATIONS)/$${next}_$(NAME).down.sql >/dev/null; \
	echo "gen แล้ว: $(MIGRATIONS)/$${next}_$(NAME).{up,down}.sql — อ่านก่อน commit ทุกครั้ง"

.PHONY: prisma-validate
prisma-validate: node_modules ## เช็คว่า schema.prisma ยังถูกไวยากรณ์
	DATABASE_URL="$(DB_URL)" $(PRISMA) validate

.PHONY: prisma-format
prisma-format: node_modules ## จัดฟอร์แมต schema.prisma
	DATABASE_URL="$(DB_URL)" $(PRISMA) format

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
