-- "DNA": il like dei pensieri, con un'identità tutta sua (doppia elica).
-- Funziona come un mi-piace classico: un utente può lasciare al massimo un DNA
-- per pensiero (toggle). La cancellazione a cascata rimuove i DNA se il pensiero
-- o l'utente vengono eliminati.
CREATE TABLE IF NOT EXISTS dna_likes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pensiero_id UUID REFERENCES pensieri(id) ON DELETE CASCADE,
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (pensiero_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_dna_likes_pensiero ON dna_likes(pensiero_id);
