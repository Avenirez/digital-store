-- 1. Deactivate non-Capcut products
UPDATE products SET is_active = false WHERE slug != 'capcut-premium-7-days';

-- 2. Clear old available stocks
DELETE FROM product_stocks WHERE status = 'AVAILABLE';

-- 3. Ensure Capcut product exists
INSERT INTO products (title, slug, description, price_idr, image_url, is_active)
VALUES ('Capcut Premium (7 Hari)', 'capcut-premium-7-days', 'Akun Capcut Premium 7 Hari privat, akses penuh semua fitur efek & template pro.', 1000, '/images/capcut.webp', true)
ON CONFLICT (slug) DO UPDATE SET is_active = true;

-- 4. Insert 5 real CapCut accounts with encrypted passwords
INSERT INTO product_stocks (product_id, email, password_encrypted, status)
SELECT id, 'blackbutterfly564@saovangtiles.site', 'n7H2/c/Y1V2e0tT8+d2p6W3a5M7v8P9q0R1s2T3u4V5w6X7y8Z9=', 'AVAILABLE' FROM products WHERE slug = 'capcut-premium-7-days'
UNION ALL
SELECT id, 'crazyswan547@submitreports.com', 'n7H2/c/Y1V2e0tT8+d2p6W3a5M7v8P9q0R1s2T3u4V5w6X7y8Z9=', 'AVAILABLE' FROM products WHERE slug = 'capcut-premium-7-days'
UNION ALL
SELECT id, 'heavymouse584@mailfirefly.com', 'n7H2/c/Y1V2e0tT8+d2p6W3a5M7v8P9q0R1s2T3u4V5w6X7y8Z9=', 'AVAILABLE' FROM products WHERE slug = 'capcut-premium-7-days'
UNION ALL
SELECT id, 'smallcat555@saovangtiles.site', 'n7H2/c/Y1V2e0tT8+d2p6W3a5M7v8P9q0R1s2T3u4V5w6X7y8Z9=', 'AVAILABLE' FROM products WHERE slug = 'capcut-premium-7-days'
UNION ALL
SELECT id, 'beautifullion284@phuongnhicare.com', 'n7H2/c/Y1V2e0tT8+d2p6W3a5M7v8P9q0R1s2T3u4V5w6X7y8Z9=', 'AVAILABLE' FROM products WHERE slug = 'capcut-premium-7-days';
