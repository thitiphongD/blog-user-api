# Blog User API

[![CI](https://github.com/thitiphongD/blog-user-api/actions/workflows/ci.yml/badge.svg)](https://github.com/thitiphongD/blog-user-api/actions/workflows/ci.yml)

REST API สำหรับ blog — สมัคร/ล็อกอิน เขียน blog เขียนคอมเมนต์
โพสต์อ่านได้ทุกคน แก้ได้เฉพาะเจ้าของ

Go + Echo + GORM + PostgreSQL — schema ของ DB จัดการด้วย Prisma

## เริ่มยังไง

ต้องมี **Docker** กับ **Go 1.25+** (Node ต้องมีเฉพาะตอนจะแก้ schema ของ DB)

```bash
git clone https://github.com/thitiphongD/blog-user-api.git
cd blog-user-api
make setup     # ก๊อป .env ให้ + go mod download
make up        # ยก postgres + api ขึ้นทั้งชุด
```

ประมาณ 30 วินาทีก็พร้อม เช็คด้วย:

```bash
curl localhost:8080/health
```

migration รันเองตอน app start ไม่ต้องสั่งอะไรเพิ่ม

ลองยิงดู:

```bash
curl -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Daew","email":"daew@example.com","password":"password123"}'

curl -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"daew@example.com","password":"password123"}'
```

หรือ import `postman/blog-user-api.postman_collection.json` เข้า Postman แล้วกด Run
ทั้งชุดได้เลย (token เก็บให้อัตโนมัติ ไม่ต้องก๊อปเอง)

**Swagger UI**: <http://localhost:8080/swagger/index.html>

## API

base path `/api/v1` — `*` = ต้องแนบ `Authorization: Bearer <token>`

| Method | Path | |
|---|---|---|
| POST | `/auth/register` | สมัคร (ไม่ auto-login) |
| POST | `/auth/login` | ล็อกอิน ได้ access + refresh token |
| POST | `/auth/refresh` | ต่ออายุ session |
| POST | `/auth/logout` | ออกจากระบบ |
| GET | `/auth/me` * | ข้อมูลตัวเอง |
| GET | `/users` `/users/:id` * | ดู user |
| GET | `/blogs` `/blogs/:id` | อ่าน blog — public |
| POST | `/blogs` * | เขียน blog |
| PUT / DELETE | `/blogs/:id` * | แก้/ลบ — เจ้าของเท่านั้น |
| GET | `/blogs/:id/comments` | อ่านคอมเมนต์ — public |
| POST | `/blogs/:id/comments` * | เขียนคอมเมนต์ |
| PUT / DELETE | `/comments/:id` * | แก้/ลบ — เจ้าของเท่านั้น |
| GET | `/health` | เช็คว่า API + DB พร้อม |

`GET /blogs` รับ `?page= &limit= &search= &sort= &order= &user_id=`

ทุก response หน้าตาเหมือนกันหมด:

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

## คำสั่งที่ใช้บ่อย

`make` เฉยๆ จะโชว์ทั้งหมด

| | |
|---|---|
| `make up` / `make down` / `make clean` | ยก stack / หยุด / หยุด+ล้างข้อมูล |
| `make logs` / `make ps` / `make psql` | ดู log / สถานะ / เข้า psql |
| `make dev` | รันบนเครื่องแบบ hot reload |
| `make test` / `make cover` | เทสต์ |
| `make prisma-diff NAME=x` | แก้ `schema.prisma` แล้ว gen migration ใบใหม่ (ต้องมี Node) |
| `make smoke` | ยิง Postman collection ทั้งชุด |
| `make ci` | รันทุกอย่างที่ CI รัน — ผ่านอันนี้ก่อนค่อย push |

รันบนเครื่องแทน docker ก็ได้ (`make dev`) แค่ต้องมี postgres อยู่ก่อน — `make up` แล้ว
`docker compose stop api` ก็ได้ postgres เปล่าๆ ไว้ใช้

## แก้ schema ของ DB

schema เขียนไว้ที่ `prisma/schema.prisma` ที่เดียว แก้เสร็จแล้ว:

```bash
make prisma-diff NAME=add_blog_tags   # gen migration ใบใหม่จากส่วนต่าง
```

Prisma ทำหน้าที่ **แปลง schema เป็น SQL อย่างเดียว** ตัวที่รัน migration จริงยังเป็น
golang-migrate ที่อ่าน SQL ฝังใน binary เหมือนเดิม — ไม่มี prisma client ในโค้ด
และ image ที่ deploy ยังเป็น Go binary ล้วน ไม่มี Node

**อ่าน SQL ที่ gen ออกมาก่อน commit ทุกครั้ง** — Prisma ไม่รู้จัก partial index กับตาราง
`schema_migrations` ของ golang-migrate เหตุผลและกับดักที่เจอมาแล้วอยู่ใน
[ARCHITECTURE.md](ARCHITECTURE.md#migration)

## Config

ค่าทั้งหมดอยู่ใน `.env` (`make setup` ก๊อปมาจาก `.env.example` ให้แล้ว) ค่า default
ใช้ได้ทันทีสำหรับ dev มีอันเดียวที่ต้องแก้ก่อนใช้จริงคือ **`JWT_SECRET`**

`DB_HOST` ตั้ง `localhost` ไว้สำหรับรันบนเครื่อง — ตอนรันด้วย docker compose มัน override
เป็น `postgres` ให้เอง **ไม่ต้องแก้**

## อยากรู้ลึกกว่านี้

[`ARCHITECTURE.md`](ARCHITECTURE.md) — โครงสร้าง layer, กฎที่ยึดทั้งโปรเจกต์, การออกแบบ
session/refresh token, error handling, เทสต์, lint, CI และเหตุผลเบื้องหลังแต่ละอย่าง
