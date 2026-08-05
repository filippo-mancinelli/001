-- Pensieri su chiunque, anche su chi non ha un account.
-- Il soggetto di un pensiero può ora essere:
--   * un utente registrato  -> subject_id valorizzato, subject_name NULL
--   * un nome libero        -> subject_name valorizzato, subject_id NULL
--
-- Un nome libero non ha profilo, non riceve notifiche e non può accettare le
-- richieste "sono curioso": quel ruolo passa all'autore del pensiero.
-- Idempotente: gira anche su DB già popolati.

ALTER TABLE pensieri ADD COLUMN IF NOT EXISTS subject_name VARCHAR(64);

-- Esattamente uno dei due riferimenti deve essere valorizzato.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint
                 WHERE conrelid = to_regclass('pensieri')
                   AND conname = 'pensieri_soggetto_check') THEN
    ALTER TABLE pensieri ADD CONSTRAINT pensieri_soggetto_check
      CHECK ((subject_id IS NOT NULL) <> (subject_name IS NOT NULL));
  END IF;
END $$;

-- La UNIQUE (author_id, subject_id) di 001_init non copre i nomi liberi, perché
-- subject_id è NULL e NULL non è confrontabile: serve un indice parziale sul
-- nome (case-insensitive) per mantenere un solo pensiero per autore/soggetto.
CREATE UNIQUE INDEX IF NOT EXISTS idx_pensieri_autore_nome
  ON pensieri (author_id, LOWER(subject_name))
  WHERE subject_id IS NULL;

-- La versione "personale" (quella che si sblocca con "sono curioso") era
-- riconoscibile solo perché audience_id coincideva con il soggetto. Con un nome
-- libero non c'è nessun utente a cui indirizzarla, quindi il suo audience_id è
-- NULL come quello della versione pubblica: serve un discriminante esplicito.
ALTER TABLE versioni_pensiero ADD COLUMN IF NOT EXISTS personale BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE versioni_pensiero vp
SET personale = TRUE
FROM pensieri p
WHERE vp.pensiero_id = p.id
  AND vp.audience_id IS NOT NULL
  AND vp.audience_id = p.subject_id
  AND NOT vp.personale;

-- Con audience_id NULL la UNIQUE (pensiero_id, audience_id) non scatta: senza
-- questo indice un pensiero su un nome libero potrebbe accumulare più versioni
-- pubbliche o più versioni personali.
CREATE UNIQUE INDEX IF NOT EXISTS idx_versioni_pensiero_default
  ON versioni_pensiero (pensiero_id, personale)
  WHERE audience_id IS NULL;
