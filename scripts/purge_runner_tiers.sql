-- Purga manual de tiers pagos y suscripciones del rol `corredor` (Gap 42).
--
-- NO es una migración automática: `AutoMigrate` no ejecuta este archivo. Es una
-- limpieza de una sola vez, transaccional e idempotente, para bases de desarrollo
-- y testing. No correrla contra local/demo/producción sin revisar antes.
--
-- Conserva el tier base interno del corredor (menor `hierarchy`, normalmente
-- `base`), porque `user_roles.tier_id` es NOT NULL y `GET /auth/permissions`
-- sigue resolviéndolo. Reasigna cualquier asignación de corredor al base antes
-- de borrar los tiers pagos, y no toca entrenador, roles globales ni el base.
--
-- Efecto colateral esperado: `payments.installment_id` tiene FK `ON DELETE SET
-- NULL`, así que los pagos ya conciliados conservan su fila pero pierden el
-- vínculo a la cuota purgada.
--
-- Uso:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/purge_runner_tiers.sql

BEGIN;

DO $$
DECLARE
  v_role_id bigint;
  v_base_tier_id bigint;
BEGIN
  SELECT id INTO v_role_id
  FROM roles
  WHERE name = 'corredor' AND deleted_at IS NULL;

  IF v_role_id IS NULL THEN
    RAISE NOTICE 'rol corredor inexistente: no hay nada que purgar';
    RETURN;
  END IF;

  SELECT id INTO v_base_tier_id
  FROM tiers
  WHERE role_id = v_role_id AND deleted_at IS NULL
  ORDER BY hierarchy ASC, id ASC
  LIMIT 1;

  IF v_base_tier_id IS NULL THEN
    RAISE EXCEPTION 'el rol corredor no tiene tier base activo; purga abortada';
  END IF;

  UPDATE user_roles
  SET tier_id = v_base_tier_id, updated_at = NOW()
  WHERE role_id = v_role_id AND tier_id <> v_base_tier_id;

  DELETE FROM installments i
  USING user_role_tier_subscriptions s
  WHERE i.subscription_id = s.id AND s.role_id = v_role_id;

  DELETE FROM tier_permissions tp
  USING tiers t
  WHERE tp.tier_id = t.id
    AND t.role_id = v_role_id
    AND t.id <> v_base_tier_id;

  DELETE FROM user_role_tier_subscriptions
  WHERE role_id = v_role_id;

  DELETE FROM tiers
  WHERE role_id = v_role_id AND id <> v_base_tier_id;

  RAISE NOTICE 'purga del corredor completada (role_id=%, base_tier_id=%)',
    v_role_id, v_base_tier_id;
END $$;

COMMIT;

-- Verificación posterior: el base sigue existiendo, no quedan tiers pagos ni
-- suscripciones del corredor y ninguna asignación apunta fuera del base.
SELECT
  (SELECT count(*) FROM tiers t
     JOIN roles r ON r.id = t.role_id
    WHERE r.name = 'corredor' AND t.deleted_at IS NULL) AS tiers_corredor,
  (SELECT count(*) FROM user_role_tier_subscriptions s
     JOIN roles r ON r.id = s.role_id
    WHERE r.name = 'corredor') AS suscripciones_corredor,
  (SELECT count(*) FROM user_roles ur
     JOIN roles r ON r.id = ur.role_id
     JOIN tiers t ON t.id = ur.tier_id
    WHERE r.name = 'corredor'
      AND (t.role_id <> ur.role_id OR t.deleted_at IS NOT NULL)) AS asignaciones_fuera_del_base;
