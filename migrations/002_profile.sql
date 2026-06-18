-- Personalizzazione profilo, immagine, status e impostazioni generali.
-- Idempotente: usa ADD COLUMN IF NOT EXISTS così gira anche su DB esistenti.

ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(64);
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url   TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS status       VARCHAR(140);
ALTER TABLE users ADD COLUMN IF NOT EXISTS presence     VARCHAR(16) DEFAULT 'online';
ALTER TABLE users ADD COLUMN IF NOT EXISTS location     VARCHAR(120);
ALTER TABLE users ADD COLUMN IF NOT EXISTS website      VARCHAR(255);
