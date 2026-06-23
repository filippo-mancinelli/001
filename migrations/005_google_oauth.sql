-- Supporto login/registrazione con Google (OAuth).
-- Gli account creati via Google non hanno password locale, quindi
-- password_hash diventa nullable e aggiungiamo google_id per collegare
-- l'identità Google all'utente.

ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id VARCHAR(255) UNIQUE;
