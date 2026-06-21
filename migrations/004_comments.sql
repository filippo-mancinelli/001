-- Commenti ai pensieri.
-- Un commento appartiene a un pensiero ed è scritto da un utente. È visibile a
-- chiunque possa vedere il pensiero a cui è collegato. La cancellazione a
-- cascata rimuove i commenti se il pensiero o l'autore vengono eliminati.
CREATE TABLE IF NOT EXISTS commenti (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pensiero_id UUID REFERENCES pensieri(id) ON DELETE CASCADE,
    author_id   UUID REFERENCES users(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_commenti_pensiero ON commenti(pensiero_id, created_at);
