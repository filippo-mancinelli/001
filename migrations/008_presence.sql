-- Presence reale e automatica: lo stato online/offline non è più un valore
-- impostato a mano dall'utente ma viene derivato dall'ultimo "battito" (last_seen)
-- registrato dal client mentre l'app/tab è aperta. Se l'ultimo battito è recente
-- l'utente è online, altrimenti offline. Al logout last_seen viene azzerato così
-- l'utente risulta offline immediatamente.
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_seen TIMESTAMPTZ;

-- indice per le query di presenza (ordinamenti/filtri futuri sullo stato online).
CREATE INDEX IF NOT EXISTS idx_users_last_seen ON users (last_seen);
