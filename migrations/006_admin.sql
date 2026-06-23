-- Ruolo amministratore per la moderazione dei contenuti.
-- Un admin può eliminare liberamente qualsiasi pensiero o commento per moderare
-- contenuti sensibili o vietati. Gli admin vengono designati tramite la variabile
-- d'ambiente ADMIN_EMAILS (vedi bootstrap in cmd/server/main.go).
-- Idempotente: usa ADD COLUMN IF NOT EXISTS così gira anche su DB esistenti.
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;
