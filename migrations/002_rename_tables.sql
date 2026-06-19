-- Rebranding: rinomina le tabelle da "thoughts/thought_versions" a "pensieri/versioni_pensiero"
-- Usa ALTER TABLE ... RENAME per preservare dati, indici e constraint esistenti.
--
-- Attenzione: 001_init.sql usa CREATE TABLE IF NOT EXISTS con i nomi NUOVI, quindi su un DB
-- pre-rebranding può aver creato tabelle pensieri/versioni_pensiero "nuove" accanto a quelle
-- vecchie (thoughts/thought_versions). La migrazione deve distinguere due situazioni:
--
--   1) Le tabelle nuove sono VUOTE  -> sono solo gli scheletri creati da 001_init: si possono
--      eliminare e si rinominano le tabelle vecchie (che contengono i dati reali).
--   2) Le tabelle nuove hanno DATI  -> l'applicazione gira già sul nuovo schema e i dati live
--      stanno in pensieri/curious_requests. In questo caso thoughts/thought_versions sono
--      residui legacy da eliminare: NON bisogna assolutamente fare DROP delle tabelle nuove,
--      altrimenti si distruggono i dati live e si orfanizzano le righe di curious_requests
--      (causa dell'errore "violates foreign key constraint ... (SQLSTATE 23503)").

DO $$
DECLARE
  pensieri_has_data boolean := false;
BEGIN
  -- La tabella "pensieri" (nuovo schema) contiene già dati live?
  IF to_regclass('pensieri') IS NOT NULL THEN
    EXECUTE 'SELECT EXISTS (SELECT 1 FROM pensieri)' INTO pensieri_has_data;
  END IF;

  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'thoughts') THEN
    IF pensieri_has_data THEN
      -- Caso 2: pensieri/versioni_pensiero contengono i dati live. Eliminiamo solo i
      -- residui legacy senza toccare il nuovo schema. CASCADE rimuove eventuali FK
      -- ancora collegate a thoughts (es. una vecchia curious_requests.thought_id).
      DROP TABLE IF EXISTS thought_versions CASCADE;
      DROP TABLE IF EXISTS thoughts CASCADE;
    ELSE
      -- Caso 1: pensieri/versioni_pensiero sono gli scheletri vuoti creati da 001_init.
      -- Li eliminiamo (CASCADE per le FK dipendenti, es. curious_requests appena creata)
      -- e rinominiamo le tabelle vecchie preservandone i dati.
      DROP TABLE IF EXISTS versioni_pensiero CASCADE;
      DROP TABLE IF EXISTS pensieri CASCADE;
      ALTER TABLE thoughts RENAME TO pensieri;
      IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'thought_versions') THEN
        ALTER TABLE thought_versions RENAME TO versioni_pensiero;
      END IF;
    END IF;
  END IF;

  -- Rinomina le colonne thought_id -> pensiero_id dove ancora presenti (tabelle vecchie).
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name = 'versioni_pensiero' AND column_name = 'thought_id') THEN
    ALTER TABLE versioni_pensiero RENAME COLUMN thought_id TO pensiero_id;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name = 'curious_requests' AND column_name = 'thought_id') THEN
    ALTER TABLE curious_requests RENAME COLUMN thought_id TO pensiero_id;
  END IF;

  -- Ripristina la FK di curious_requests verso pensieri se mancante. La guardia rende la
  -- migrazione idempotente e sicura anche su DB nuovi (dove la FK creata da 001_init esiste già).
  IF to_regclass('curious_requests') IS NOT NULL
     AND to_regclass('pensieri') IS NOT NULL
     AND EXISTS (SELECT 1 FROM information_schema.columns
                 WHERE table_name = 'curious_requests' AND column_name = 'pensiero_id')
     AND NOT EXISTS (SELECT 1 FROM pg_constraint
                     WHERE conrelid = to_regclass('curious_requests')
                       AND contype = 'f'
                       AND confrelid = to_regclass('pensieri')) THEN
    -- Rimuovi eventuali righe orfane (pensiero_id non presente in pensieri) prima di
    -- aggiungere la FK, altrimenti ADD CONSTRAINT fallirebbe con SQLSTATE 23503. Sono
    -- richieste già "rotte" che puntano a pensieri inesistenti.
    DELETE FROM curious_requests
    WHERE pensiero_id IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM pensieri p WHERE p.id = curious_requests.pensiero_id);

    ALTER TABLE curious_requests
      ADD CONSTRAINT curious_requests_pensiero_id_fkey
      FOREIGN KEY (pensiero_id) REFERENCES pensieri(id) ON DELETE CASCADE;
  END IF;
END $$;
