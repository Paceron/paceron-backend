-- Purga manual de tiers pagos y suscripciones del rol `corredor` (Gap 42).
--
-- NO es una migración automática: `AutoMigrate` no ejecuta este archivo. Es una
-- limpieza de una sola vez, transaccional e idempotente. Solo bases descartables
-- de test. Nunca local/demo/producción.
--
-- Conserva el tier base interno del corredor (menor `hierarchy` activa con
-- `payment_required=false`). `user_roles.tier_id` es NOT NULL porque toda
-- asignación de rol exige un tier. `GET /auth/permissions` NO usa ese caché:
-- resuelve el tier efectivo por la suscripción `active` del ledger o, si no hay,
-- por `FindLowestByRole` del rol. Con el base presente, los permisos del
-- corredor siguen resolviendo base. Reasigna las asignaciones al base antes de
-- borrar los tiers pagos; no toca entrenador, roles globales ni el base.
--
-- Efecto colateral esperado: en bases migradas por la app existe la FK
-- `payments.installment_id ON DELETE SET NULL`, así que los pagos ya conciliados
-- conservan su fila pero pierden el vínculo a la cuota purgada. En bases sin esa
-- FK, `installment_id` conserva el valor viejo.
--
-- Uso:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/purge_runner_tiers.sql

BEGIN;

DO $$
DECLARE
  v_role_id bigint;
  v_base_tier_id bigint;
  v_tier_count int;
  v_tier_is_free boolean;
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
  WHERE role_id = v_role_id
    AND deleted_at IS NULL
    AND payment_required = FALSE
  ORDER BY hierarchy ASC, id ASC
  LIMIT 1;

  IF v_base_tier_id IS NULL THEN
    RAISE EXCEPTION 'el rol corredor no tiene un tier base gratuito activo; purga abortada (no se conservará un tier pago como base)';
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

  SELECT count(*), bool_and(NOT payment_required)
  INTO v_tier_count, v_tier_is_free
  FROM tiers
  WHERE role_id = v_role_id AND deleted_at IS NULL;

  IF v_tier_count <> 1 THEN
    RAISE EXCEPTION 'verificación de purga falló: el corredor tiene % tiers activos (se esperaba exactamente 1)', v_tier_count;
  END IF;
  IF v_tier_is_free IS DISTINCT FROM TRUE THEN
    RAISE EXCEPTION 'verificación de purga falló: el tier restante del corredor no es gratuito';
  END IF;
  IF EXISTS (
    SELECT 1
    FROM user_roles ur
    WHERE ur.role_id = v_role_id
      AND NOT EXISTS (
        SELECT 1
        FROM tiers t
        WHERE t.id = ur.tier_id
          AND t.role_id = v_role_id
          AND t.deleted_at IS NULL
          AND t.payment_required = FALSE
      )
  ) THEN
    RAISE EXCEPTION 'verificación de purga falló: hay asignaciones de corredor fuera del tier base gratuito';
  END IF;
  IF EXISTS (
    SELECT 1 FROM user_role_tier_subscriptions WHERE role_id = v_role_id
  ) THEN
    RAISE EXCEPTION 'verificación de purga falló: quedan suscripciones de tier del corredor';
  END IF;

  RAISE NOTICE 'purga del corredor completada (role_id=%, base_tier_id=%)',
    v_role_id, v_base_tier_id;
END $$;

COMMIT;

-- Resumen informativo (la verificación que aborta/revierte ya corrió en el DO).
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
      AND (t.role_id <> ur.role_id OR t.deleted_at IS NOT NULL OR t.payment_required)) AS asignaciones_fuera_del_base;
