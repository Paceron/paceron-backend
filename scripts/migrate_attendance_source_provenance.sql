-- Migración de assistencias: origen de la asistencia + integridad referencial.
--
-- CONTRATO IMPORTANTE (decisión explícita del change gestion-asistencia-entrenador):
-- esta migración es MANUAL y OBLIGATORIA. NO la aplica `AutoMigrate`.
--
-- Motivo: en este proyecto los modelos GORM no declaran asociaciones `constraint:`
-- (grep "constraint:" cmd/api/domains/dbs/*.go → 0 resultados), así que
-- `AutoMigrate` genera índices y columnas pero NINGUNA foreign key. Consecuencia
-- directa: una base creada sólo por `AutoMigrate` (CI, `make test-db-up`, la máquina
-- de un compañero, un staging nuevo) queda SIN la FK de más abajo, y las
-- asistencias pueden apuntar a un session_instance inexistente sin dar error.
--
-- Por eso la verificación de la FK (tarea 0.8 del change) no puede ser un test de
-- comportamiento contra la base: correría contra una base sin la restricción y
-- fallaría siempre. Lo que sí se automatiza es que este archivo no se pierda —
-- ver cmd/api/daos/attendance_migration_test.go.
--
-- Idempotencia: el paso 3 (borrado de huérfanas) y el 4 (backfill) son seguros de
-- repetir. Los pasos 2 y 5 (agregar columna / agregar constraint) NO lo son: si ya
-- se aplicaron, la sentencia falla con "already exists". Para eso los pasos 2 y 5
-- van envueltos en un DO block que chequea el catálogo y no hace nada si el objeto
-- ya está. Correr el archivo entero las veces que haga falta.
--
-- Cuándo: una vez por base, antes de deployar el código que usa `source`.
--hacer backup antes del paso 3: borra asistencias huérfanas de forma irreversible.

-- ---------------------------------------------------------------------------
-- 1. Preview: cuántas asistencias huérfanas hay. CORRER SOLO, sin escribir.
--    Si devuelve 0, el paso 3 no va a borrar nada.
-- ---------------------------------------------------------------------------
SELECT a.id, a.team_id, a.training_session_id, a.user_id
FROM attendances a
LEFT JOIN session_instances si ON si.id = a.training_session_id
WHERE si.id IS NULL;

-- ---------------------------------------------------------------------------
-- 2. Columnas de provenance. `registered_by_user_id` va SIN FK a propósito: es
--    auditoría de "quién lo registró", y un usuario borrado no debe poder
--    borrarle el historial a un corredor.
-- ---------------------------------------------------------------------------
ALTER TABLE attendances ADD COLUMN IF NOT EXISTS source text NULL;
ALTER TABLE attendances ADD COLUMN IF NOT EXISTS registered_by_user_id bigint NULL;

-- ---------------------------------------------------------------------------
-- 3. Limpieza de huérfanas. IRREVERSIBLE: borra asistencias cuyo
--    session_instance ya no existe. El paso 1 tiene que dar 0 filas antes de
--    seguir; si da más, son datos reales y hay que decidir con el equipo.
-- ---------------------------------------------------------------------------
DELETE FROM attendances a
WHERE a.training_session_id NOT IN (SELECT id FROM session_instances);

-- ---------------------------------------------------------------------------
-- 4. Backfill. El ÚNICO escritor previo a este change era el registro por QR
--    (services/attendance_service.go Register), así que toda asistencia histórica
--    es de origen 'qr'. NO 'manual': el alta manual es nueva en este change.
--
--    Requiere el paso 3 antes: si quedara alguna huérfana, el NOT NULL de abajo
--    fallaría y es mejor que falle acá que dejar la columna a medio migrar.
-- ---------------------------------------------------------------------------
UPDATE attendances SET source = 'qr' WHERE source IS NULL;
ALTER TABLE attendances ALTER COLUMN source SET NOT NULL;

-- ---------------------------------------------------------------------------
-- 5. FK a session_instances. SIN `ON DELETE`, o sea RESTRICT a propósito:
--    CASCADE borraría en silencio el histórico de asistencias de un corredor.
--
--    Acoplamiento conocido: esta restricción empieza a fallar si alguna vez se
--    expone un DELETE de session_instances o se relaja isCalendarDayClosed. Hoy
--    no hay ruta DELETE /session-instances/:id, DeleteDay rechaza días cerrados y
--    calendar_service.go excluye el borrado de la instancia al cancelar un día.
--    Si eso cambia, revisar esta constraint. Ver la nota de acoplamiento en
--    openspec/changes/gestion-asistencia-entrenador/design.md.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fk_attendances_session_instance'
  ) THEN
    ALTER TABLE attendances
      ADD CONSTRAINT fk_attendances_session_instance
      FOREIGN KEY (training_session_id) REFERENCES session_instances(id);
  END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 6. Índice único que necesita el ON CONFLICT del bulk upsert.
--    Tiene que existir y ser UNIQUE y NO PARCIAL sobre las tres columnas, en ese
--    orden. Si ya existe (es lo normal: venís de una versión con el índice puesto
--    a mano) el bloque no hace nada.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes
    WHERE tablename = 'attendances'
      AND indexdef ~ 'UNIQUE'
      AND indexdef ~ 'team_id.*training_session_id.*user_id'
      AND indexdef NOT LIKE '%WHERE%'
  ) THEN
    CREATE UNIQUE INDEX uq_att_team_session_user
      ON attendances (team_id, training_session_id, user_id);
  END IF;
END $$;

-- ---------------------------------------------------------------------------
-- Verificación: las 4 tienen que dar lo que dice el comentario.
-- ---------------------------------------------------------------------------
SELECT
  count(*) FILTER (WHERE source IS NULL)              AS source_null,        -- 0
  count(DISTINCT source)                               AS valores_source,     -- 1 ('qr')
  EXISTS (SELECT 1 FROM information_schema.columns
          WHERE table_name = 'attendances' AND column_name = 'source'
            AND is_nullable = 'NO')                    AS source_not_null,    -- t
  EXISTS (SELECT 1 FROM pg_constraint
          WHERE conname = 'fk_attendances_session_instance') AS fk_existe     -- t
ORDER BY 1;
