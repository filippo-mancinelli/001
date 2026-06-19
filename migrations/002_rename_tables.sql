-- Rebranding: rinomina le tabelle da "thoughts/thought_versions" a "pensieri/versioni_pensiero"
-- Usa ALTER TABLE ... RENAME per preservare dati, indici e constraint esistenti.

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'thoughts') THEN
    -- 001_init.sql usa CREATE TABLE IF NOT EXISTS con i nomi nuovi, quindi su un DB
    -- esistente può aver già creato pensieri/versioni_pensiero/curious_requests vuote.
    -- Le tabelle nuove e vuote vanno eliminate prima di rinominare quelle vecchie.
    -- Serve CASCADE perché curious_requests ha una FK verso pensieri: senza CASCADE
    -- il DROP fallisce con "cannot drop table pensieri because other objects depend on it"
    -- (SQLSTATE 2BP01). CASCADE rimuove solo la FK dipendente, non la tabella
    -- curious_requests (che ricostruiremo la FK più sotto).
    DROP TABLE IF EXISTS versioni_pensiero CASCADE;
    DROP TABLE IF EXISTS pensieri CASCADE;
    ALTER TABLE thoughts RENAME TO pensieri;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'thought_versions') THEN
    ALTER TABLE thought_versions RENAME TO versioni_pensiero;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name = 'versioni_pensiero' AND column_name = 'thought_id') THEN
    ALTER TABLE versioni_pensiero RENAME COLUMN thought_id TO pensiero_id;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name = 'curious_requests' AND column_name = 'thought_id') THEN
    ALTER TABLE curious_requests RENAME COLUMN thought_id TO pensiero_id;
  END IF;

  -- Ripristina la FK di curious_requests verso pensieri se il DROP ... CASCADE
  -- qui sopra l'ha rimossa. La guardia rende la migrazione idempotente e sicura
  -- anche su DB nuovi (dove la FK creata da 001_init.sql esiste già).
  IF to_regclass('curious_requests') IS NOT NULL
     AND EXISTS (SELECT 1 FROM information_schema.columns
                 WHERE table_name = 'curious_requests' AND column_name = 'pensiero_id')
     AND NOT EXISTS (SELECT 1 FROM pg_constraint
                     WHERE conrelid = to_regclass('curious_requests')
                       AND contype = 'f'
                       AND confrelid = to_regclass('pensieri')) THEN
    ALTER TABLE curious_requests
      ADD CONSTRAINT curious_requests_pensiero_id_fkey
      FOREIGN KEY (pensiero_id) REFERENCES pensieri(id) ON DELETE CASCADE;
  END IF;
END $$;
