-- "Corona": un amministratore può coronare i pensieri che ritiene migliori.
-- I pensieri coronati vengono messi in primo piano nel feed e mostrati con un
-- bordo dorato leggero e una piccola corona.
-- Idempotente: usa ADD COLUMN IF NOT EXISTS così gira anche su DB esistenti.
ALTER TABLE pensieri ADD COLUMN IF NOT EXISTS crowned BOOLEAN NOT NULL DEFAULT FALSE;
