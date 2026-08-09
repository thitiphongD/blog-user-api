# Blog User API

[![CI](https://github.com/thitiphongD/blog-user-api/actions/workflows/ci.yml/badge.svg)](https://github.com/thitiphongD/blog-user-api/actions/workflows/ci.yml)

REST API สำหรับ blog — สมัคร/ล็อกอิน แล้วเขียน blog ของตัวเอง อ่านได้ทุกคน แก้ได้เฉพาะเจ้าของ
เขียนด้วย Go + Echo + GORM + PostgreSQL

โฟกัสของโปรเจกต์นี้คือ **โครงสร้างและ code quality** ไม่ใช่จำนวน feature

## Architecture

Layered แบ่งหน้าที่ชัด ไล่จากบนลงล่าง คุยข้ามชั้นไม่ได้

```
Handler      รับ request → validate → เรียก service → คืน response  (ห้ามมี business logic)
   ↓
Service      business logic + ownership check                       (รู้จัก interface ของ repo เท่านั้น)
   ↓
Repository   คุย database อย่างเดียว                                 (ไม่รู้จัก HTTP)
   ↓
PostgreSQL
```

กฎที่ยึดทั้งโปรเจกต์:

- ทุก method ของ service/repository รับ `ctx context.Context` เป็น arg ตัวแรก
- Service ผูกกับ **interface** ของ repository ไม่ผูกกับ struct → mock ได้ ทดสอบไม่ต้องต่อ DB
- **Error ของ GORM ห้ามทะลุขึ้น service** — repo แปลงเป็น domain error ใน `internal/apperr` ก่อนคืน
- **Model ห้ามหลุดออก response** ต้อง map เป็น DTO เสมอ (กัน password hash หลุด)
- Schema มี source of truth เดียวคือไฟล์ SQL ใน `internal/migrate/migrations/` — **GORM ห้าม `AutoMigrate`**
- JSON กับ DB column เป็น `snake_case` ทั้งระบบ
- UUID กับ timestamp เซ็ตฝั่ง app (`uuid.New()` + GORM autoCreateTime) ไม่ใช้ DB default

## Tech Stack

| ตัว | ใช้ทำอะไร |
|---|---|
| Go 1.25 | ภาษา |
| Echo v4 | HTTP framework |
| GORM | query (ไม่ใช้ทำ migration) |
| golang-migrate | migration, embed ไว้ใน binary |
| PostgreSQL 16 | database |
| golang-jwt | auth token |
| bcrypt | hash password (cost 12) |
| go-playground/validator | validate request |
| Air | hot reload ตอน dev |
| Docker Compose | รันทั้งชุด |

## Folder Structure

```
cmd/server/main.go          entry point

internal/
  config/                   อ่าน env
  database/                 ต่อ postgres + retry ตอน boot
  apperr/                   domain error (ErrNotFound, ErrEmailTaken, ...)
  migrate/                  รัน migration ตอน start
    migrations/             ไฟล์ SQL — source of truth ของ schema
  handler/                  auth / blog / user / health
  service/                  business logic
  repository/               คุย DB (ตั้งชื่อตาม entity: user, blog)
  dto/request/              payload ขาเข้า
  dto/response/             payload ขาออก
  model/                    GORM model
  middleware/               request id / logger / recover / cors / jwt
  routes/                   ประกอบ route
  validator/                validator + error translator
  auth/                     JWT sign/verify + bcrypt (ไม่รู้จัก echo)
  response/                 envelope กลาง + global error handler

docs/                       swagger
```

migrations อยู่ใต้ `internal/migrate/` เพราะ `//go:embed` ห้ามมี `..` — embed ไฟล์นอก
directory ของ package ตัวเองไม่ได้ ถ้าวางไว้ `db/migrations/` ที่ root คือ compile ไม่ผ่าน

ไม่มี `internal/utils/` — package ตั้งชื่อตามหน้าที่ ไม่ใช่ถังขยะ

## Environment

ก๊อป `.env.example` เป็น `.env` แล้วแก้ค่า

```env
APP_PORT=8080
APP_ENV=development

# docker compose override เป็น postgres ให้เอง
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=blog_user_api
DB_SSLMODE=disable

DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=5m

SERVER_READ_TIMEOUT=10s
SERVER_WRITE_TIMEOUT=10s
SERVER_IDLE_TIMEOUT=60s

JWT_SECRET=change-me-in-production
JWT_EXPIRE_HOURS=24
```

- `JWT_SECRET` เปลี่ยนก่อน deploy จริงด้วย อย่าปล่อยค่า default
- `DB_HOST` ตั้ง `localhost` ไว้สำหรับรันบนเครื่อง เวลารันด้วย compose ตัว service `api`
  จะ override เป็น `postgres` ให้ — **ไม่ต้องแก้ `.env` เอง** และห้ามเปลี่ยนเป็น `postgres`
  ไม่งั้นโหมด local จะต่อไม่ติด
- pool กับ timeout ไม่ใช่ของเสริม — `database/sql` default คือ open conns ไม่จำกัด และ
  `http.Server` ที่ไม่ตั้ง timeout คือเปิดรับ slowloris
- comment ในไฟล์นี้ต้องอยู่บรรทัดของตัวเอง อย่าเขียนต่อท้ายค่า — `Makefile` include
  ไฟล์นี้ไปใช้ประกอบ `DB_URL` ค่าจะติดขยะเอา

## Installation

ต้องมี Go 1.25+ และ Docker

```bash
git clone https://github.com/thitiphongD/blog-user-api.git
cd blog-user-api
make setup     # ก๊อป .env + go mod download
make tools     # ลง golangci-lint / swag / migrate / air ตามเวอร์ชันที่ปักไว้
```

## Makefile

คำสั่งทั้งหมดในเอกสารนี้มีทางลัดใน `Makefile` — `make` เฉยๆ จะโชว์ทั้งหมด

| คำสั่ง | ทำอะไร |
|---|---|
| `make up` / `make down` / `make clean` | ยก stack / หยุด / หยุด+ล้าง volume |
| `make logs` / `make ps` / `make psql` | ดู log / สถานะ / เข้า psql |
| `make dev` | รันบนเครื่องแบบ hot reload |
| `make test` / `make test-race` / `make cover` | เทสต์ |
| `make lint` / `make swag` | lint / gen swagger |
| `make smoke` | ยิง Postman collection ด้วย newman |
| `make migrate-create NAME=x` | สร้าง migration ใหม่ |
| **`make ci`** | **รันทุกอย่างที่ CI รัน — ผ่านอันนี้ก่อนค่อย push** |

เวอร์ชันเครื่องมือปักไว้ใน `Makefile` ให้ตรงกับ CI จะได้ไม่เจอ "เครื่องผมผ่านนะ"

## Run

### Docker (ง่ายสุด รันทั้งชุด)

```bash
docker compose up -d --build     # postgres + api
docker compose logs -f api
docker compose down              # หยุด
docker compose down -v           # หยุด + ล้าง volume ของ DB
```

API ขึ้นที่ `http://localhost:8080` เช็คว่าพร้อมหรือยังด้วย `curl localhost:8080/health`

api รอ postgres ผ่าน healthcheck ก่อนถึงจะสตาร์ท (`depends_on: condition: service_healthy`)
และในโค้ดยัง retry ตอนต่อ DB อีกชั้น — ขาดอันใดอันหนึ่ง migration จะตายตอน boot

binary build ด้วย `CGO_ENABLED=0` เพราะ runtime image เป็น alpine (musl) ถ้าไม่ปิด CGO
จะได้ binary ที่ link glibc แล้วรันไม่ขึ้น ขึ้น `no such file or directory` ทั้งที่ไฟล์อยู่ตรงนั้น

### Local (dev)

เปิด postgres ก่อน แล้วรัน API บนเครื่อง

```bash
docker compose up -d postgres
air                              # hot reload
# หรือไม่ใช้ air
go run ./cmd/server
```

## Migration

Migration ถูก embed อยู่ใน binary และ **รันอัตโนมัติตอน app start** ปกติไม่ต้องสั่งเอง

เวลาจะเพิ่มตารางใหม่:

```bash
# ติดตั้ง cli ครั้งเดียว
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# สร้างไฟล์คู่ up/down
migrate create -ext sql -dir internal/migrate/migrations -seq create_comments
```

สั่งเองเมื่อจำเป็น (เช่น rollback):

```bash
export DB_URL="postgres://postgres:postgres@localhost:5432/blog_user_api?sslmode=disable"
export MIG=internal/migrate/migrations

migrate -path $MIG -database "$DB_URL" up
migrate -path $MIG -database "$DB_URL" down 1
migrate -path $MIG -database "$DB_URL" version
```

แก้ไฟล์ migration ที่รันไปแล้วห้ามทำ — สร้างไฟล์ใหม่เสมอ

## API

base path `/api/v1` — `*` = ต้องแนบ `Authorization: Bearer <token>`

### Health

| Method | Path | คำอธิบาย |
|---|---|---|
| GET | `/health` | เช็คว่า API + DB พร้อม (ไม่มี `/api/v1` prefix) |

### Auth

| Method | Path | คำอธิบาย |
|---|---|---|
| POST | `/auth/register` | สมัครสมาชิก — คืน 201 + user (ไม่ auto-login) |
| POST | `/auth/login` | ล็อกอิน คืน JWT |
| GET | `/auth/me` * | ข้อมูลตัวเอง |

สมัครเสร็จต้องยิง `/auth/login` ต่อเพื่อเอา token — สมัครกับล็อกอินเป็นคนละเรื่อง

กติกา validate: `name` 2–100, `email` ต้องเป็น email, `password` **8–72 ตัว**
(72 เพราะ bcrypt กินได้แค่ 72 bytes เกินกว่านั้นมันตัดทิ้งเงียบๆ), `title` 1–200, `content` ห้ามว่าง

### User

| Method | Path | คำอธิบาย |
|---|---|---|
| GET | `/users` * | list user (pagination) |
| GET | `/users/:id` * | ดู user รายคน |

### Blog

| Method | Path | คำอธิบาย |
|---|---|---|
| GET | `/blogs` | list blog — public |
| GET | `/blogs/:id` | ดู blog รายอัน — public |
| POST | `/blogs` * | สร้าง |
| PUT | `/blogs/:id` * | แก้ (replace เต็ม ส่ง title+content ครบ) — **เจ้าของเท่านั้น** |
| DELETE | `/blogs/:id` * | ลบ (soft delete) คืน 200 ไม่ใช่ 204 — **เจ้าของเท่านั้น** |

`/blogs` เป็น public แต่ `/users` ปิด → `author` ใน `BlogResponse` มีแค่ `id` กับ `name`
**ไม่มี email** ไม่งั้นปิดประตูหน้าแล้วเปิดประตูหลัง

### Query params ของ `GET /blogs`

| param | default | หมายเหตุ |
|---|---|---|
| `page` | 1 | ต่ำสุด 1 |
| `limit` | 20 | **max 100** เกินจะถูก clamp |
| `search` | - | ค้นจาก title/content (ILIKE ไม่สนตัวพิมพ์) |
| `sort` | `created_at` | whitelist: `created_at`, `title` |
| `order` | `desc` | `asc` / `desc` |
| `user_id` | - | กรองตามผู้เขียน |

`GET /users` ใช้ `page` / `limit` แบบเดียวกัน (ไม่มี search/sort/filter)

`sort` / `order` / `user_id` ที่ส่งค่านอก whitelist คืน **400 พร้อมบอกว่ารับค่าอะไรได้บ้าง**
ไม่เงียบๆ ถอยไปใช้ default — ส่งผิดควรได้รู้ และ ORDER BY ประกอบจาก constant เท่านั้น
ไม่เอา string จาก query มาต่อ SQL

### ตัวอย่าง

```bash
# สมัคร
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Daew","email":"daew@example.com","password":"password123"}'

# ล็อกอิน เอา token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"daew@example.com","password":"password123"}'

# สร้าง blog
curl -X POST http://localhost:8080/api/v1/blogs \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <token>' \
  -d '{"title":"Hello","content":"First post"}'

# list พร้อม pagination + search + sort
curl 'http://localhost:8080/api/v1/blogs?page=1&limit=20&search=golang&sort=created_at&order=desc'
```

## Postman

`postman/blog-user-api.postman_collection.json` — import ไฟล์เดียวจบ ไม่ต้องมี environment แยก
(ตัวแปรอยู่ในตัว collection แล้ว)

รันเรียงจากบนลงล่างได้เลย `token` / `user_id` / `blog_id` เก็บจาก response อัตโนมัติ
ไม่ต้องก๊อป token มือ

5 โฟลเดอร์ 27 request — Health / Auth / Blog / User และ **Negative** ที่รวมเคสซึ่ง
*ต้องพัง*: register ซ้ำ 409, validate ไม่ผ่าน 422, JSON พัง 400, รหัสผิด 401,
email ที่ไม่มีในระบบ 401 (ต้องได้ข้อความเดียวกับรหัสผิด), ไม่มี token 401,
id ไม่ใช่ UUID 400, sort/order นอก whitelist 400 — ถ้าอันไหนดันผ่านแปลว่ามีปัญหา

รันทั้งชุดแบบไม่เปิด Postman:

```bash
docker compose up -d
npx newman run postman/blog-user-api.postman_collection.json
```

รันซ้ำได้เรื่อยๆ — Register รับทั้ง 201 และ 409 เลยไม่พังตอนรันรอบสอง

## Response Format

ทุก response หน้าตาเหมือนกันหมด ออกจาก `internal/response` ที่เดียว
และมี `request_id` + `timestamp` ติดมาทุกก้อน (เอาไว้ตาม log)
เวลาทุกตัวเป็น UTC ลงท้ายด้วย `Z` ทั้งหมด ไม่มีตัวไหนโผล่มาเป็น local offset

```json
{
  "status": 200,
  "success": true,
  "message": "Success",
  "data": {},
  "request_id": "01J...",
  "timestamp": "2026-08-09T10:00:00Z"
}
```

แบบมี pagination:

```json
{
  "status": 200,
  "success": true,
  "message": "Success",
  "data": [],
  "pagination": { "page": 2, "limit": 20, "total": 120, "total_page": 6 },
  "request_id": "01J...",
  "timestamp": "2026-08-09T10:00:00Z"
}
```

แบบ error:

```json
{
  "status": 422,
  "success": false,
  "message": "Validation failed",
  "errors": { "title": "Title is required" },
  "request_id": "01J...",
  "timestamp": "2026-08-09T10:00:00Z"
}
```

| Status | ใช้เมื่อ |
|---|---|
| 200 / 201 | สำเร็จ |
| 400 | JSON พัง / param ผิด format |
| 401 | ไม่มี token หรือ token ใช้ไม่ได้ / login ผิด |
| 403 | login แล้ว แต่ไม่ใช่เจ้าของ |
| 404 | ไม่เจอข้อมูล |
| 409 | email ซ้ำ |
| 422 | JSON ถูก แต่ validate ไม่ผ่าน |
| 500 | พังฝั่ง server (ไม่คาย error จริงออก เอา `request_id` ไปตามใน log แทน) |

**400 กับ 422 ไม่ใช่อันเดียวกัน** — 400 คือ parse ไม่ได้, 422 คือ parse ได้แต่ค่าไม่ผ่านกติกา

status พวกนี้ไม่ได้ map กระจายตาม handler — handler แค่ `return err` แล้ว global error handler
แปลง domain error จาก `internal/apperr` เป็น HTTP ที่เดียว

## Log

`log/slog` เป็น JSON ทั้งหมด ทุกบรรทัดมี `request_id` ตัวเดียวกับที่อยู่ใน response
เจอ 500 ก็เอา `request_id` จาก response ไป grep ใน log ได้เลย

```json
{"time":"...","level":"WARN","msg":"request","request_id":"sTJU...","method":"GET","path":"/api/v1/blogs","status":400,"latency_ms":0.027,"remote_ip":"::1"}
```

level ผูกกับ status: 5xx = ERROR, 4xx = WARN, ที่เหลือ INFO
`request_id` เดินไปกับ `context.Context` ด้วย (`internal/logging`) service กับ repository
เลย log ได้โดยไม่ต้องรู้จัก echo

## Test

```bash
go test ./...
go test -race ./...
go test -cover ./internal/service/...
```

Service layer ทดสอบด้วย mock repository เขียนมือ (เพราะ service ผูก interface) ไม่ต้องมี DB
ไม่ต้องลง mock generator เพิ่ม — mock เป็น struct ที่มี func field ธรรมดา method ไหนไม่ได้เซ็ต
แล้วถูกเรียกจะ panic ซึ่งใช้จับได้เลยว่า ownership check กัน `Update`/`Delete` ได้จริง

ที่คุมไว้: register hash password, email ซ้ำ, login สำเร็จ/รหัสผิด/email ไม่มีในระบบ
(ต้องได้ `ErrInvalidCredential` ไม่ใช่ `ErrNotFound` ไม่งั้นบอกใบ้ว่า email ไหนสมัครไว้),
ownership ของ update/delete, transaction ของ create, pagination clamp, sort/order whitelist,
การคำนวณ `total_page`

## Lint

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
golangci-lint run
```

config อยู่ที่ `.golangci.yml` (schema v2) — `errorlint` เปิดไว้เพราะโปรเจกต์นี้พึ่ง
`errors.Is` กับ domain error ถ้าใครเผลอเขียน `err == apperr.ErrNotFound` จะโดนจับ

## CI

`.github/workflows/ci.yml` รัน build / vet / `go test -race` / golangci-lint
แล้วเช็คด้วยว่า `docs/` ที่ commit ไว้ยังตรงกับ annotation ในโค้ด — แก้ handler แล้วลืม
`swag init` จะ fail ตรงนั้น

## Docs

Swagger UI อยู่ที่ `http://localhost:8080/swagger/index.html` spec ดิบที่ `/swagger/doc.json`

spec generate จาก annotation บน handler แล้ว **commit ลงรีโป** (`docs/`) เพราะ build
ต้องใช้ ไม่งั้นคนโคลนไปต้องลง swag ก่อนถึงจะ build ได้ แก้ annotation แล้วต้อง gen ใหม่:

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.4
swag init -g cmd/server/main.go -o docs --parseInternal --parseDependency
```

`--parseDependency` จำเป็น ไม่ใส่แล้ว swag หา type ใน `internal/dto/response` ไม่เจอ
