# README — High-Performance Digital Account Store

Proyek e-commerce produk digital (Email + Password) berkinerja tinggi yang dibangun menggunakan **Go (Golang)**, **Astro v4**, **PostgreSQL**, **Redis**, dan payment gateway **Duitku**.

---

## 🏗️ Struktur Proyek

```
/my-digital-store
├── /backend            # REST API Server berbasis Go (Chi Router)
│   ├── cmd/api/main.go # Entry point server API
│   ├── internal/
│   │   ├── config/     # Loader env (Duitku, Postgres, Redis, Crypto, dll)
│   │   ├── database/   # Koneksi PostgreSQL (pgxpool) & Redis client
│   │   ├── handler/    # HTTP Handlers (Product, Order, Webhook, Admin, Restock)
│   │   ├── middleware/ # Middleware Redis Rate Limiter & CORS/AdminAuth
│   │   ├── repository/ # Layer Query Database
│   │   └── service/    # Layanan (AES-256-GCM, Duitku, Resend Email, Telegram, Expire Worker)
│   ├── migrations/     # Migrasi SQL DDL PostgreSQL
│   ├── .env.example    # Template variabel lingkungan
│   ├── go.mod
│   └── Makefile        # Perintah build & kompilasi binary
│
└── /frontend           # Aplikasi Frontend Astro (Tailwind CSS, Cloudflare Pages Ready)
    ├── src/
    │   ├── components/ # Component UI (Header, Footer, ProductCard, CopyBox, RestockForm)
    │   ├── layouts/    # BaseLayout.astro (Glassmorphism & Theme)
    │   └── pages/      # Halaman (index.astro, produk/[slug].astro, lacak.astro, checkout/[order_id].astro)
    ├── astro.config.mjs
    ├── tailwind.config.mjs
    └── package.json
```

---

## ⚡ Fitur Utama

1. **Race Condition Prevention & Inventory Locking**:
   - Menggunakan query PostgreSQL `FOR UPDATE SKIP LOCKED` pada stok `AVAILABLE` saat reservasi pesanan untuk mencegah *double selling*.
2. **Integrasi Payment Gateway Duitku**:
   - Pembuatan transaksi Duitku V2 Inquiry.
   - Verifikasi callback webhook aman menggunakan signature HMAC-SHA256.
3. **Penyimpanan Kredensial Terenkripsi**:
   - Password akun dienkripsi menggunakan standar **AES-256-GCM** sebelum disimpan ke database.
4. **On-The-Fly Credential Download (.txt)**:
   - File kredensial `.txt` di-generate secara *in-memory* langsung ke HTTP Response writer tanpa menyimpan file di disk server.
5. **Guest Order Lookup**:
   - Pembeli dapat melacak pesanan dan melihat kredensial tanpa perlu login dengan memasukkan Order ID & Email.
6. **Bulk Stock Import (Admin)**:
   - Endpoint admin `POST /api/v1/admin/stocks/bulk` yang dilindungi header `X-Admin-Key` untuk mengimpor stok masal format `email|password|additional_info`.
7. **Notifikasi Restock (Resend API)**:
   - Form pendaftaran email otomatis saat stok 0. Pekerja otomatis mengirimkan email notifikasi saat admin mengimpor stok baru.
8. **Telegram Bot Alerts**:
   - Notifikasi otomatis & *asynchronous* saat ada transaksi baru yang sukses atau saat stok produk di bawah 5 item.
9. **Redis Rate Limiting**:
   - Membatasi percobaan checkout maksimal 3 request per 10 menit per IP.
10. **Background Auto-Expire Worker**:
    - Pekerja Go ticker setiap 1 menit yang secara otomatis mengubah status pesanan `PENDING` > 15 menit menjadi `EXPIRED` dan mengembalikan stok tereservasi kembali ke `AVAILABLE`.

---

## 🚀 Panduan Menjalankan Proyek

### 1. Persiapan Backend (Go)
1. Buat database PostgreSQL 16 & instansi Redis 7.
2. Salin file `.env.example` ke `.env` di dalam folder `/backend`:
   ```bash
   cp backend/.env.example backend/.env
   ```
3. Jalankan migrasi database SQL:
   ```bash
   make migrate DATABASE_URL="postgres://user:pass@localhost:5432/digitalstore?sslmode=disable"
   ```
4. Jalankan API Server Go:
   ```bash
   cd backend
   go run cmd/api/main.go
   ```

### 2. Persiapan Frontend (Astro)
1. Install dependensi Node.js:
   ```bash
   cd frontend
   npm install
   ```
2. Jalankan mode pengembangan Astro:
   ```bash
   npm run dev
   ```
3. Buka browser di `http://localhost:4321`.

---

## 📦 Rekomendasi Ruang Kerja (Workspace)
Disarankan untuk mengatur direktori berikut sebagai active workspace Anda:
`C:\Users\Awan\.gemini\antigravity-ide\scratch\my-digital-store`
