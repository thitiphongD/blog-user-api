# Architecture

เอกสารนี้คือเหตุผลเบื้องหลังโปรเจกต์ — วิธี setup กับ endpoint อยู่ใน [README](README.md)

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
- Schema มี source of truth เดียวคือ `prisma/schema.prisma` → gen เป็น SQL ลง
  `internal/migrate/migrations/` — **GORM ห้าม `AutoMigrate`**
- JSON กับ DB column เป็น `snake_case` ทั้งระบบ
- **`ORDER BY` ที่ใช้คู่กับ paging ต้องมี tiebreaker เสมอ** — ต่อ `, id` ปิดท้ายทุกที่
  ค่าที่ซ้ำกันทำให้ postgres คืนลำดับไม่คงที่ระหว่างหน้า แถวหลุดหรือโผล่ซ้ำได้
- UUID กับ timestamp เซ็ตฝั่ง app (`uuid.New()` + GORM autoCreateTime) ไม่ใช้ DB default

## Tech Stack

| ตัว | ใช้ทำอะไร |
|---|---|
| Go 1.25 | ภาษา |
| Echo v4 | HTTP framework |
| GORM | query (ไม่ใช้ทำ migration) |
| golang-migrate | รัน migration, embed ไว้ใน binary |
| Prisma 7 | gen SQL migration จาก `schema.prisma` (เครื่องมือ dev ไม่ติดไปกับ image) |
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
    migrations/             ไฟล์ SQL ที่ Prisma gen ให้ — ตัวที่รันจริง
  handler/                  auth / blog / comment / user / health
  logging/                  พา request_id ไปกับ context.Context
  service/                  business logic
  repository/               คุย DB (ตั้งชื่อตาม entity: user, blog, comment, refresh_token)
  dto/request/              payload ขาเข้า
  dto/response/             payload ขาออก
  model/                    GORM model
  middleware/               request id / logger / recover / cors / jwt / rate limit
  routes/                   ประกอบ route
  validator/                validator + error translator
  auth/                     JWT sign/verify + bcrypt (ไม่รู้จัก echo)
  response/                 envelope กลาง + global error handler

docs/                       swagger

prisma/
  schema.prisma             source of truth ของ schema
  extras.sql                SQL ที่ Prisma เขียนไม่ได้ (partial index)
prisma.config.ts            datasource + external table ของ Prisma 7
```

migrations อยู่ใต้ `internal/migrate/` เพราะ `//go:embed` ห้ามมี `..` — embed ไฟล์นอก
directory ของ package ตัวเองไม่ได้ ถ้าวางไว้ `db/migrations/` ที่ root คือ compile ไม่ผ่าน

ไม่มี `internal/utils/` — package ตั้งชื่อตามหน้าที่ ไม่ใช่ถังขยะ

## Environment

ค่าทั้งหมดอยู่ใน `.env.example` — ที่ควรรู้:

- `JWT_SECRET` เปลี่ยนก่อน deploy จริงด้วย อย่าปล่อยค่า default
- `DB_HOST` ตั้ง `localhost` ไว้สำหรับรันบนเครื่อง เวลารันด้วย compose ตัว service `api`
  จะ override เป็น `postgres` ให้ — **ไม่ต้องแก้ `.env` เอง** และห้ามเปลี่ยนเป็น `postgres`
  ไม่งั้นโหมด local จะต่อไม่ติด
- pool กับ timeout ไม่ใช่ของเสริม — `database/sql` default คือ open conns ไม่จำกัด และ
  `http.Server` ที่ไม่ตั้ง timeout คือเปิดรับ slowloris
- comment ในไฟล์นี้ต้องอยู่บรรทัดของตัวเอง อย่าเขียนต่อท้ายค่า — `Makefile` include
  ไฟล์นี้ไปใช้ประกอบ `DB_URL` ค่าจะติดขยะเอา

## Docker

api รอ postgres ผ่าน healthcheck ก่อนถึงจะสตาร์ท (`depends_on: condition: service_healthy`)
และในโค้ดยัง retry ตอนต่อ DB อีกชั้น — **ขาดอันใดอันหนึ่ง migration จะตายตอน boot**

สามอันนี้พลาดแล้วเสียเวลา debug ยาว:

1. **`CGO_ENABLED=0`** — runtime image เป็น alpine (musl) ไม่ปิด CGO จะได้ binary ที่ link
   glibc แล้วรันไม่ขึ้น ขึ้น `no such file or directory` ทั้งที่ไฟล์อยู่ตรงนั้น
   (pgx เป็น pure Go ปิดได้สบาย)
2. **`DB_HOST=postgres` override ใน compose** — `.env` ตั้ง `localhost` ไว้สำหรับรัน local
   ถ้าไม่ override container จะต่อหาตัวเอง
3. **healthcheck ใช้ `wget` ไม่ใช่ `curl`** — alpine ไม่มี curl มีแต่ busybox wget

image สุดท้าย ~50MB รันด้วย user `app` (uid 10001) ไม่ใช่ root

## Migration

Migration ถูก embed อยู่ใน binary และ **รันอัตโนมัติตอน app start** ปกติไม่ต้องสั่งเอง

schema เขียนไว้ที่ `prisma/schema.prisma` แล้วให้ Prisma แปลงเป็น SQL ให้:

```
prisma/schema.prisma  ← แก้ตรงนี้ที่เดียว
      │
      │ make prisma-diff NAME=add_tags
      ▼
internal/migrate/migrations/*.sql
      │
      │ //go:embed
      ▼
   app start → migrate.Up()
```

Prisma ในโปรเจกต์นี้เป็น **ตัว gen SQL อย่างเดียว** — ไม่มี prisma client ไม่มีตาราง
`_prisma_migrations` และไม่มี Node ใน image ที่ deploy จริง (`.dockerignore` กัน `node_modules/`
กับ `prisma/` ออกจาก build context) คนรัน migration ยังเป็น golang-migrate เหมือนเดิม

| คำสั่ง | ทำอะไร |
|---|---|
| `make prisma-diff NAME=x` | diff DB จริงกับ `schema.prisma` แล้ว gen migration ใบถัดไป |
| `make prisma-init` | gen ใหม่ทั้งหมดจากศูนย์ ล้าง `migrations/` ทิ้ง — ใช้ตอนตั้งต้นเท่านั้น |
| `make prisma-validate` / `make prisma-format` | เช็ค/จัดฟอร์แมต `schema.prisma` |
| `make migrate-up` / `migrate-down` / `migrate-version` | สั่ง golang-migrate ตรงๆ |
| `make migrate-create NAME=x` | migration เปล่าไว้เขียน SQL มือ (เช่น backfill ข้อมูล) |

**สามอย่างที่ Prisma ทำให้ไม่ได้ ต้องรู้ไว้:**

1. **partial index** (`CREATE INDEX ... WHERE deleted_at IS NULL`) — ไม่มีไวยากรณ์รองรับ
   ของพวกนี้เลยอยู่ที่ `prisma/extras.sql` แล้วตัว gen ต่อท้าย migration ทุกใบให้
   (เป็น `IF NOT EXISTS` ใบที่ไม่ได้แตะ index เลยไม่พัง) จะแก้ definition ของ index เดิม
   ต้องเขียน `DROP INDEX` ในไฟล์ migration เอง
2. **`schema_migrations`** คือสมุดจดของ golang-migrate ที่ Prisma ไม่รู้จัก ถ้าไม่ประกาศเป็น
   external table ใน `prisma.config.ts` มันจะ gen `DROP TABLE schema_migrations` ออกมาให้ —
   ประกาศไว้แล้ว แต่นี่คือเหตุผลว่าทำไมต้องอ่าน SQL ที่ gen ออกมาก่อน commit ทุกครั้ง
3. `UNIQUE` ที่ Prisma ออกให้เป็น **unique index** ไม่ใช่ unique constraint (บังคับความ unique
   ได้เท่ากัน แต่ชื่อใน catalog คนละชนิด) และ `@db.Timestamptz(6)` จะเขียน typmod ลงไปตรงๆ
   ต่างจาก `TIMESTAMPTZ` เปล่าๆ ที่เขียนมือ — พฤติกรรมเท่ากันเพราะ default ของ postgres คือ 6

แก้ไฟล์ migration ที่รันไปแล้วห้ามทำ — สร้างไฟล์ใหม่เสมอ (`make prisma-init` ล้างทิ้งหมด
ใช้ได้เฉพาะตอนที่ยังไม่มี DB ไหนวิ่งอยู่จริง)

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
| POST | `/auth/login` | ล็อกอิน คืน access token + refresh token |
| POST | `/auth/refresh` | ต่ออายุ session — หมุน refresh token ทุกครั้ง |
| POST | `/auth/logout` | เพิกถอนเฉพาะ session ที่ยื่นมา |
| GET | `/auth/me` * | ข้อมูลตัวเอง |

สมัครเสร็จต้องยิง `/auth/login` ต่อเพื่อเอา token — สมัครกับล็อกอินเป็นคนละเรื่อง

### Session

**access token อายุ 15 นาที** (`JWT_ACCESS_TTL`) เพราะเป็น JWT ที่เพิกถอนไม่ได้ —
ออกไปแล้วก็ใช้ได้จนหมดอายุ ต่อให้เจ้าของกด logout แล้วก็ตาม เลยตั้งให้สั้น

**refresh token อายุ 7 วัน** (`JWT_REFRESH_TTL`) เก็บเป็นแถวใน DB จึงเพิกถอนได้จริง
เก็บแค่ SHA-256 ของตัว token ไม่เก็บของดิบ — DB หลุดแล้วเอาไป login ต่อไม่ได้
(ใช้ SHA-256 ไม่ใช่ bcrypt เพราะ token สุ่มมา 256 bit ไม่มีอะไรให้เดา และต้องหาแถวด้วย hash ตรงๆ)

**หมุนทุกครั้งที่ refresh** — ยิง `/auth/refresh` แล้วใบเก่าถูกเพิกถอนทันที ได้ใบใหม่มาแทน
ทั้งหมดอยู่ใน transaction เดียว

การหมุนพึ่ง `UPDATE ... WHERE id = ? AND revoked_at IS NULL` เป็น compare-and-swap แล้ว
**เช็ค `RowsAffected` ด้วย** — การอ่าน token อยู่นอก transaction สองคำขอที่ถือใบเดียวกัน
ยิงพร้อมกันจึงเห็น `revoked_at` เป็น null ได้ทั้งคู่ ถ้าปล่อยให้ update ที่ไม่โดนแถวไหน
คืน `nil` เฉยๆ จะออก token ใหม่ให้ทั้งสองคำขอ = ใบเดียวแตกเป็นสองสายที่ใช้ได้จริง
และ reuse detection ข้างล่างก็ไม่มีวันทำงานเพราะไม่มีใครเอาใบเก่ามายิงซ้ำ
ตัวที่แพ้ตอนนี้ได้ 401 แล้ว transaction rollback (`TestRevokeConcurrentOnlyOneWins` ยืนยันของจริง)

`/auth/logout` กลืน error ตัวนี้ทิ้งตั้งใจ — เพิกถอนไม่โดนแถวแปลว่ามีคนชิงทำไปแล้ว
ปลายทางคือ session ตายเหมือนกัน ไม่มีเหตุผลให้ client เห็น error

**กวาดของหมดอายุทิ้งเป็นระยะ** (`REFRESH_PRUNE_INTERVAL` ค่าเริ่ม `1h` ใส่ `0` คือปิด) —
`refresh_tokens` ได้แถวใหม่ทุกครั้งที่มีคน login หรือ refresh และไม่มีอะไรลบให้
ปล่อยไว้คือโตไม่มีเพดาน goroutine ใน `main` เรียก `PruneExpiredTokens` ตาม ticker
ผูกกับ ctx ตัวเดียวกับ shutdown

**ตัดที่ `expires_at` ไม่ใช่ `revoked_at` ตั้งใจ** — ใบที่ถูกเพิกถอนแล้วแต่ยังไม่หมดอายุ
คือของที่ใช้จับการเอา token ไปใช้ซ้ำ ลบเร็วไปเมื่อไหร่คนขโมย token มายิงจะได้ 401 เฉยๆ
แทนที่ระบบจะรู้ตัวแล้วตัดทุก session ทิ้ง

**จับ token ที่ถูกใช้ซ้ำ** — ถ้ามีคนเอาใบที่เพิกถอนไปแล้วมายิงอีก แปลว่ามีคนถืออยู่สองมือ
ระบบจะ **ตัดทุก session ของ user คนนั้นทิ้ง** แล้วบังคับให้ login ใหม่ทั้งหมด
เป็นราคาที่ยอมจ่ายเพื่อไม่ให้คนที่ขโมย token ไปอยู่ในระบบต่อได้เงียบๆ

### Rate limit

`POST /auth/login` กับ `POST /auth/register` จำกัด **10 request/นาที/IP**
(`RATE_LIMIT_AUTH_PER_MINUTE` — ใส่ `0` คือปิด) เกินแล้วได้ **429** ในรูป envelope เดียวกับที่อื่น

ไม่ครอบ `/auth/refresh` เพราะ token สุ่ม 256 bit เดาไม่ได้อยู่แล้ว และ client ปกติยิง refresh
บ่อยกว่า login มาก จำกัดไปด้วยจะพังการใช้งานจริงเปล่าๆ

เก็บสถานะไว้ในหน่วยความจำของ process — พอสำหรับ instance เดียว **ถ้าขยายเป็นหลาย replica
ต้องย้ายไปเก็บที่ส่วนกลาง (Redis)** ไม่งั้นโควตาจะคูณตามจำนวน pod

คีย์ที่ใช้นับคือ `c.RealIP()` ซึ่ง **ต้องปัก `e.IPExtractor = echo.ExtractIPDirect()` ด้วย** —
ค่า default ของ echo คืออ่าน `X-Forwarded-For` แล้วค่อยถอยไป `RemoteAddr` แปลว่าใครก็ยัด
header ปลอมใหม่ทุก request แล้วได้โควตาใหม่เรื่อยๆ (และปลอม `remote_ip` ใน access log ได้ด้วย)
ตอนนี้ไม่มี proxy อยู่หน้า service วันไหนมีค่อยเปลี่ยนเป็น `ExtractIPFromXFFHeader` พร้อมระบุ
range ที่เชื่อถือได้ — มีเทสต์ที่ `cmd/server/main_test.go` กันไว้ไม่ให้หลุดกลับไป

### CORS

`CORS_ALLOWED_ORIGINS` คั่นด้วย comma ค่าเริ่มคือ `*` (เปิดหมด) เพื่อให้ dev รันได้เลย —
ตอน boot ถ้ายังเป็น `*` จะมี log warn เตือน **ก่อน deploy จริงต้องระบุ origin ให้ชัด**

ความเสี่ยงไม่สูงเท่า API ที่ใช้ cookie เพราะ token อยู่ใน `Authorization` header
เว็บอื่นต้องมี token อยู่ในมือก่อนถึงจะยิงแทนผู้ใช้ได้ แต่ไม่ใช่เหตุผลให้เปิดทิ้งไว้

### Validation

`name` 2–100, `email` ต้องเป็น email, `password` **8–72 ตัว**, `title` 1–200,
`content` ห้ามว่าง, คอมเมนต์ 1–2000

**72 ไม่ใช่เลขมั่ว** — bcrypt รับได้แค่ 72 bytes แต่พฤติกรรมสองฝั่งไม่เหมือนกัน:
`GenerateFromPassword` **คืน error** ถ้าเกิน (ไม่กั้นแล้ว register จะกลายเป็น 500)
ส่วน `CompareHashAndPassword` **ยังตัดที่ 72 อยู่** (ไม่กั้นฝั่ง login แล้วรหัสยาว 72 ตัว
จะยอมรับส่วนท้ายอะไรต่อก็ได้) เลยต้องปัก `max=72` ทั้งสองฝั่ง

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

### Comment

| Method | Path | คำอธิบาย |
|---|---|---|
| GET | `/blogs/:id/comments` | list comment ของ blog — public, เรียงเก่าไปใหม่ |
| POST | `/blogs/:id/comments` * | เขียนคอมเมนต์ |
| PUT | `/comments/:id` * | แก้ — **เจ้าของคอมเมนต์เท่านั้น** |
| DELETE | `/comments/:id` * | ลบ (soft delete) — **เจ้าของคอมเมนต์เท่านั้น** |

อ่าน/สร้างซ้อนใต้ `/blogs/:id` เพราะคอมเมนต์ไม่มีความหมายถ้าไม่มีโพสต์
ส่วนแก้/ลบอ้างถึงตัวคอมเมนต์ตรงๆ ไม่ต้องรู้ว่ามันอยู่ใต้ blog ไหน

เรียง**เก่าไปใหม่** ต่างจาก blog ที่เรียงใหม่ไปเก่า — บทสนทนาต้องอ่านไล่จากบนลงล่าง

**blog ที่ถูกลบไปแล้ว คอมเมนต์ไม่ได้และอ่านคอมเมนต์ไม่ได้ (404)** — FK กันได้แค่ตอนลบถาวร
แต่ blog เป็น soft delete แถวยังอยู่ FK เลยไม่ช่วย ต้องเช็คเองที่ service

**เจ้าของ blog ลบคอมเมนต์คนอื่นไม่ได้** — เป็นเรื่อง moderation ซึ่งจงใจไม่ทำ ไม่ใช่ลืม

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

## Postman

`postman/blog-user-api.postman_collection.json` — import ไฟล์เดียวจบ ไม่ต้องมี environment แยก
(ตัวแปรอยู่ในตัว collection แล้ว)

รันเรียงจากบนลงล่างได้เลย `token` / `refresh_token` / `user_id` / `blog_id` /
`comment_id` เก็บจาก response อัตโนมัติ ไม่ต้องก๊อป token มือ

7 โฟลเดอร์ 41 request / 61 assertion — Health / Auth / Blog / Comment / User /
**Negative** / Logout และโฟลเดอร์ Negative รวมเคสซึ่ง *ต้องพัง*: register ซ้ำ 409,
validate ไม่ผ่าน 422, JSON พัง 400, รหัสผิด 401, email ที่ไม่มีในระบบ 401
(ต้องได้ข้อความเดียวกับรหัสผิด), refresh token มั่ว 401, ไม่มี token 401,
id ไม่ใช่ UUID 400, sort/order นอก whitelist 400 — ถ้าอันไหนดันผ่านแปลว่ามีปัญหา

รันทั้งชุดแบบไม่เปิด Postman: `make smoke`

Register รับทั้ง 201 และ 409 เลยรันซ้ำได้ — แต่ **รันสองรอบติดใน 1 นาทีจะเจอ 429**
ที่ login เพราะ rate limit 10/นาที/IP (นั่นคือมันทำงานถูก) รอสักนาทีแล้วรันใหม่

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
| 429 | ยิงถี่เกินโควตาของ login/register |
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
make test              # unit ทั้งหมด ไม่ต้องมี DB
make test-race         # อันเดียวกับที่ CI รัน
make cover             # coverage รายชั้น
make test-integration  # ยิง postgres จริง (ยก container ให้เอง)
```

**263 unit + 34 integration**

| ชั้น | coverage |
|---|---|
| `config` / `dto/request` / `logging` / `middleware` / `model` / `response` / `validator` | 100% |
| `handler` | 95.8% |
| `service` | 95.9% |
| `auth` | 92.9% |

ที่ไม่ถึง 100% เหลือ 2 branch ที่**วิ่งไม่ถึงผ่านทางปกติ** ตั้งใจไม่ไล่ทดสอบ:

- `middleware.UserID` ใน handler — พังได้ต่อเมื่อมีคนลืมใส่ JWT middleware ให้ route นั้น
  เก็บไว้เป็นกันชน
- `jwt.SignedString` — HS256 กับ key ที่เป็น `[]byte` ไม่มีทางพัง

บิดเทสต์ให้ถึงสองอันนี้ได้เลข 100% สวยแต่ไม่ได้ความมั่นใจเพิ่ม

`repository` ไม่มี unit test เพราะ mock พิสูจน์ SQL แทนไม่ได้ — ดูหัวข้อ integration test ข้างล่าง

Service layer ทดสอบด้วย mock repository เขียนมือ (เพราะ service ผูก interface) ไม่ต้องมี DB
ไม่ต้องลง mock generator เพิ่ม — mock เป็น struct ที่มี func field ธรรมดา method ไหนไม่ได้เซ็ต
แล้วถูกเรียกจะ panic ซึ่งใช้จับได้เลยว่า ownership check กัน `Update`/`Delete` ได้จริง

ที่คุมไว้: register hash password, email ซ้ำ, login สำเร็จ/รหัสผิด/email ไม่มีในระบบ
(ต้องได้ `ErrInvalidCredential` ไม่ใช่ `ErrNotFound` ไม่งั้นบอกใบ้ว่า email ไหนสมัครไว้),
ownership ของ update/delete, transaction ของ create, pagination clamp, sort/order whitelist,
การคำนวณ `total_page`

ทาง error ก็คุมด้วย ไม่ใช่เทสต์แต่ทางที่สำเร็จ: **DB ล่มตอน login ต้องเด้งเป็น 500 ไม่ใช่
กลายเป็น "รหัสผิด"** (ไม่งั้น DB ล่มทีไรผู้ใช้ไปนั่งรีเซ็ตรหัสกันทั้งบ้าน), `Count` พังแล้ว
ต้องไม่ยิง `FindAll` ต่อ, สร้างสำเร็จแต่อ่านกลับพังต้องไม่คืน blog ที่ไม่มี author ออกไปเงียบๆ

### เทสต์ฝั่ง HTTP

`internal/handler` ยิง request จริงผ่าน `httptest` เข้า echo ที่ประกอบ route + validator +
global error handler ของจริง — mock ลงไปถึงชั้น repository เลย เพราะ handler ผูกกับ service
struct ตรงๆ ผลพลอยได้คือ **การ map error เป็น status ถูกเช็คไปด้วย** ไม่ใช่เช็คแค่ค่าที่
handler return

ที่คุมไว้ เช่น:

- `author` ใน `/blogs` (public) **ต้องไม่มี email** — กันรั่วออกทางหลังตอน `/users` ปิดอยู่
- เจ้าของโพสต์มาจาก **token** ไม่ใช่ `user_id` ที่ client ยัดมาใน body
- ไม่ใช่เจ้าของ = **403 ไม่ใช่ 404** และ `repository.Update`/`Delete` ต้องไม่ถูกเรียกเลย
- `limit=999999` ถูก clamp ตั้งแต่ก่อนถึง repository
- 400 (JSON พัง / id ไม่ใช่ UUID / sort นอก whitelist) แยกจาก 422 (validate ไม่ผ่าน) ชัดเจน
- token ที่เซ็นด้วย secret อื่นใช้ไม่ได้
- password เกิน 72 bytes โดนปฏิเสธ ไม่ใช่ปล่อยให้ bcrypt ตัดเงียบๆ
- error ที่ไม่ได้ตั้งใจ → 500 ที่ไม่คายรายละเอียดออก response
- `health` ping DB จริงด้วย context ของ request ไม่ใช่ตอบ 200 ลอยๆ

### Integration test (ต้องมี postgres จริง)

```bash
make test-integration
```

`internal/repository` ใช้ build tag `integration` เลยไม่ปนกับ `make test` ปกติ — ที่นี่พิสูจน์
ของที่ mock พิสูจน์แทนไม่ได้ เพราะมันคือพฤติกรรมของ SQL/GORM เอง:

- **email ซ้ำ → `ErrEmailTaken`** ซึ่งพึ่ง `TranslateError: true` ใน `database.Connect`
  ใครลบบรรทัดนั้นเมื่อไหร่ 409 จะกลายเป็น 500 เงียบๆ — เทสต์นี้เป็นอย่างเดียวที่จับได้
- **soft delete จริง** — ลบแล้วหายจาก `FindByID` / `FindAll` / `Count` แต่แถวยังอยู่ใน DB
- `Update` เขียนแค่ title/content — เปลี่ยนเจ้าของผ่านทางนี้ไม่ได้
- `ILIKE` ไม่สนตัวพิมพ์ และ `Count` กับ `FindAll` ใช้ where ก้อนเดียวกัน (ไม่งั้น pagination เพี้ยน)
- `ORDER BY` ทุกคู่ของ sort/order, offset/limit
- `Preload` author มาจริง
- **transaction rollback จริง** และเขียนแล้วอ่านกลับใน tx เดียวกันเห็นของที่เพิ่งเขียน

migration ที่รันในเทสต์คือไฟล์จริงใน `internal/migrate/migrations` — schema ที่เทสต์วิ่งบน
เป็นอันเดียวกับที่ deploy

## Lint

```bash
make lint
```

config อยู่ที่ `.golangci.yml` (schema v2) เปิดไว้ประมาณ 25 ตัว ที่ตั้งใจเป็นพิเศษ:

- **`depguard`** — เอากฎ layer ที่เขียนไว้ข้างบนมาให้เครื่องบังคับแทนความจำคน
  `internal/service` / `handler` / `dto` **ห้าม import gorm**, และ `service` / `repository` /
  `auth` / `apperr` / `logging` / `model` **ห้าม import echo**
  ผิดเมื่อไหร่ lint แดงทันที ไม่ต้องรอ reviewer มาจับ
- `errorlint` — โปรเจกต์นี้พึ่ง `errors.Is` กับ domain error ใครเผลอเขียน
  `err == apperr.ErrNotFound` จะโดนจับ
- `gosec`, `nilerr`, `noctx`, `contextcheck`, `sqlclosecheck`, `rowserrcheck`
- `cyclop` / `gocognit` / `funlen` / `lll` คุมไม่ให้ function บวมเงียบๆ

ที่จงใจ**ปิด**: `govet.shadow` (`if err := f(); err != nil` เป็น idiom ปกติของ Go
ดัดให้เลิก shadow แล้วโค้ดแย่ลง) และ `govet.fieldalignment` (เรียง field ตามความหมาย
อ่านง่ายกว่าประหยัด byte) ส่วน `docs/` ข้ามทั้งโฟลเดอร์เพราะ swag gen ให้

## CI

`.github/workflows/ci.yml` มี 3 job:

- **test** — build / vet / `go test -race` + เช็คว่า `docs/` ที่ commit ไว้ยังตรงกับ
  annotation ในโค้ด (แก้ handler แล้วลืม `swag init` จะ fail ตรงนี้)
- **integration** — ยก postgres เป็น service แล้วรันเทสต์ที่ติด tag `integration`
- **lint** — golangci-lint

## Docs

Swagger UI อยู่ที่ `http://localhost:8080/swagger/index.html` spec ดิบที่ `/swagger/doc.json`

spec generate จาก annotation บน handler แล้ว **commit ลงรีโป** (`docs/`) เพราะ build
ต้องใช้ ไม่งั้นคนโคลนไปต้องลง swag ก่อนถึงจะ build ได้ แก้ annotation แล้วต้อง gen ใหม่:

```bash
make swag
```

`--parseDependency` จำเป็น ไม่ใส่แล้ว swag หา type ใน `internal/dto/response` ไม่เจอ
