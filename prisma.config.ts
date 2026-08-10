// Prisma 7 อ่าน datasource จากไฟล์นี้ ไม่ได้อ่านจาก url ใน schema.prisma แล้ว
// ใช้เฉพาะตอน gen SQL บนเครื่อง dev — ตัว API ที่ deploy จริงไม่แตะไฟล์นี้
import { defineConfig, env } from 'prisma/config'

export default defineConfig({
  schema: 'prisma/schema.prisma',
  datasource: {
    // Makefile ประกอบ DATABASE_URL จาก DB_* ใน .env ให้ก่อนเรียก
    url: env('DATABASE_URL'),
  },

  // schema_migrations เป็นสมุดจดของ golang-migrate ไม่ใช่ของ Prisma
  // ไม่ประกาศไว้ตรงนี้ prisma migrate diff จะเห็นเป็นตารางส่วนเกินแล้วสั่ง DROP ทิ้ง
  experimental: { externalTables: true },
  tables: { external: ['public.schema_migrations'] },
})
