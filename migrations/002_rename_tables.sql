-- Rebranding: rinomina le tabelle da "thoughts/thought_versions" a "pensieri/versioni_pensiero"
-- Usa ALTER TABLE ... RENAME per preservare dati, indici e constraint esistenti.

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'thoughts') THEN
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
END $$;
