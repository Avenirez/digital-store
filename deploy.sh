#!/bin/bash
# ═══════════════════════════════════════════════════════
# LEXAA STORE — Skrip Deployment Otomatis untuk Cloud VPS
# ═══════════════════════════════════════════════════════
# Jalankan: bash deploy.sh
# Prasyarat: Docker, Docker Compose, Git sudah terinstall

set -e

DOMAIN="lexaastore.cloud"
PROJECT_DIR="/var/www/digital-store"
EMAIL="admin@lexaastore.cloud"  # Email untuk Certbot (Ganti jika perlu)

echo "═══════════════════════════════════════════════"
echo "  LEXAA STORE — DEPLOYMENT SCRIPT"
echo "═══════════════════════════════════════════════"
echo ""

# ─── Step 1: Setup project directory ────────────────
echo "[1/7] 📂 Menyiapkan direktori proyek..."
if [ ! -d "$PROJECT_DIR" ]; then
    echo "  → Membuat direktori $PROJECT_DIR"
    mkdir -p "$PROJECT_DIR"
fi
cd "$PROJECT_DIR"

# ─── Step 2: Clone / Pull from GitHub ──────────────
echo "[2/7] 📦 Mengambil kode dari GitHub..."
if [ -d ".git" ]; then
    echo "  → Repository sudah ada, melakukan git pull..."
    git pull origin main
else
    echo "  → Clone repository..."
    git clone https://github.com/Avenirez/digital-store.git .
fi

# ─── Step 3: Setup .env ────────────────────────────
echo "[3/7] 🔐 Mengecek file .env..."
if [ ! -f ".env" ]; then
    if [ -f ".env.production" ]; then
        cp .env.production .env
        echo "  → Disalin dari .env.production → .env"
    else
        echo "  ❌ File .env.production tidak ditemukan!"
        exit 1
    fi
else
    echo "  → File .env sudah ada ✓"
fi

# ─── Step 4: Generate AES Key (jika masih placeholder) ─
if grep -q "GANTI_DENGAN_64_KARAKTER_HEX_RANDOM" .env 2>/dev/null; then
    NEW_AES_KEY=$(openssl rand -hex 32)
    sed -i "s/GANTI_DENGAN_64_KARAKTER_HEX_RANDOM/$NEW_AES_KEY/" .env
    echo "  → AES_KEY otomatis dibuat: $NEW_AES_KEY"
fi

if grep -q "GANTI_DENGAN_API_KEY_ADMIN_RAHASIA" .env 2>/dev/null; then
    NEW_ADMIN_KEY=$(openssl rand -base64 32 | tr -d '/+=' | head -c 32)
    sed -i "s/GANTI_DENGAN_API_KEY_ADMIN_RAHASIA/$NEW_ADMIN_KEY/" .env
    echo "  → ADMIN_API_KEY otomatis dibuat: $NEW_ADMIN_KEY"
    echo "  📝 Simpan ADMIN_API_KEY ini! Anda butuhkan untuk import stok."
fi

# ─── Step 5: Build & Start Containers ──────────────
echo "[4/7] 🐳 Membangun dan menjalankan Docker containers..."
docker compose down -v --remove-orphans 2>/dev/null || true
docker rm -f digitalstore_postgres digitalstore_redis digitalstore_backend digitalstore_frontend digitalstore_nginx digitalstore_certbot 2>/dev/null || true
docker compose up -d --build

echo "  → Menunggu containers sehat (15 detik)..."
sleep 15

# ─── Step 6: Obtain SSL Certificate ───────────────
echo "[5/7] 🔒 Mendapatkan sertifikat SSL dari Let's Encrypt..."

# Test apakah domain sudah bisa diakses
if curl -s -o /dev/null -w "%{http_code}" "http://$DOMAIN" | grep -q "200\|301\|302"; then
    echo "  → Domain $DOMAIN sudah responsif ✓"
else
    echo "  ⚠️  Domain belum merespons. Pastikan DNS sudah propagasi."
    echo "  Melanjutkan tanpa SSL untuk sekarang..."
fi

# Run Certbot
echo "  → Menjalankan Certbot..."
docker compose run --rm certbot certonly \
    --webroot \
    --webroot-path=/var/www/certbot \
    --email "$EMAIL" \
    --agree-tos \
    --no-eff-email \
    -d "$DOMAIN" \
    -d "www.$DOMAIN" \
    && SSL_SUCCESS=true || SSL_SUCCESS=false

if [ "$SSL_SUCCESS" = true ]; then
    echo "  → SSL berhasil didapatkan! ✓"

    # ─── Step 7: Switch to SSL Nginx Config ────────────
    echo "[6/7] 🔄 Mengganti konfigurasi Nginx ke HTTPS..."
    cp nginx/default.ssl.conf nginx/default.conf
    docker compose restart nginx
    echo "  → Nginx sudah menggunakan HTTPS ✓"
else
    echo "  ⚠️  SSL gagal. Website berjalan di HTTP saja untuk sekarang."
    echo "  Anda bisa retry nanti: docker compose run --rm certbot certonly --webroot --webroot-path=/var/www/certbot --email $EMAIL --agree-tos --no-eff-email -d $DOMAIN -d www.$DOMAIN"
fi

# ─── Step 7: Verification ─────────────────────────
echo "[7/7] ✅ Verifikasi deployment..."
echo ""

# Check containers
echo "Status Container:"
docker compose ps
echo ""

# Test health endpoint
HEALTH=$(curl -sk "https://$DOMAIN/api/v1/health" 2>/dev/null || curl -s "http://$DOMAIN/api/v1/health" 2>/dev/null || echo "unreachable")
echo "Health Check: $HEALTH"
echo ""

echo "═══════════════════════════════════════════════"
echo "  🎉 DEPLOYMENT SELESAI!"
echo "═══════════════════════════════════════════════"
echo ""
echo "  🌐 Website:  https://$DOMAIN"
echo "  🔧 API:      https://$DOMAIN/api/v1/health"
echo ""
echo "  📝 Langkah Selanjutnya:"
echo "  1. Buat Bot Telegram (lihat README)"
echo "  2. Edit .env → isi TELEGRAM_BOT_TOKEN & TELEGRAM_CHAT_ID"
echo "  3. Restart: docker compose restart backend"
echo "  4. Import stok produk via API admin"
echo "  5. Setup MacroDroid di HP Android"
echo ""
