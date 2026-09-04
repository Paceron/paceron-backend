# Reporte para admin de backend — tiers pagos mal configurados (testing)

> Encontrado 2026-09-03 probando el flujo de cambio de tier (`openspec/changes/cambio-tier-suscripciones/`) desde el frontend, siguiendo `01-cambio-de-tier.md`. No es un bug de la feature de pagos en sí — la feature funciona — es **data desactualizada** de antes de que existiera, más un gap de código que puede hacer que se repita.

## 1. Problema visible en frontend

Al listar los tiers de un rol (`GET /api/v1/tiers`) para ofrecer "Mejorar tier", las cards de los tiers no-base (premium corredor, medium corredor, premium entrenador, medium entrenador) aparecen **vacías** — sin badge "Tier actual" ni botón "Mejorar".

**Por qué:** el frontend filtra tiers pagos con `payment_required === true` (comportamiento correcto, documentado en el paso 3 de `01-cambio-de-tier.md`). Esos 4 tiers responden `payment_required: false` y `tier_amount: 0`, así que el frontend los trata como tiers gratis y no ofrece upgrade — es el comportamiento esperado del frontend ante datos que dicen "este tier es gratis", el problema está en el dato, no en la lógica del cliente.

## 2. Causa raíz — datos (`tiers`, DB de testing)

Los 4 tiers no-base tienen `created_at` del 2026-07-18 y 2026-09-01 — **anteriores** a que `payment_required`/`tier_amount` importaran para algo (se crearon probando el sistema de roles/tiers original, antes de la feature de pagos). Nadie los actualizó al construir `cambio-tier-suscripciones`, quedaron con los defaults del modelo.

Estado actual (`SELECT id, name, role_id, payment_required, tier_amount, hierarchy FROM tiers`):

| id | name | role_id | payment_required | tier_amount | hierarchy |
|---|---|---|---|---|---|
| 1 | base corredor | 1 (corredor) | false ✅ | 0 ✅ | 0 ❌ |
| 4 | premium corredor | 1 (corredor) | false ❌ | 0 ❌ | 0 ❌ |
| 6 | medium corredor | 1 (corredor) | false | 0 | 0 | *(ver punto 4 — no debería existir)* |
| 3 | base entrenador | 2 (entrenador) | false ✅ | 0 ✅ | 0 ❌ |
| 5 | medium entrenador | 2 (entrenador) | false ❌ | 0 ❌ | 0 ❌ |
| 2 | premium entrenador | 2 (entrenador) | false ❌ | 0 ❌ | 0 ❌ |

**Adicional, mismo origen:** `hierarchy` está en `0` en **las 6 filas**, incluidas las base. El campo existe en el modelo (`cmd/api/domains/dbs/tier.go:16`, comment `"Orden jerárquico (base=1, medium=2, premium=3)"`) pero nunca se pobló — no bloquea el flujo de cambio de tier (`ChangeTier` no compara jerarquías, solo valida mismo `role_id` + sin deuda), pero sí devuelve `hierarchy: 0` en las respuestas (`GET /tiers`, `GET /subscriptions/current`) para cualquier tier, dato incorrecto de cara al frontend si lo llega a usar para ordenar/mostrar algo.

### Fix de datos acordado (no aplicado todavía — pendiente de decisión de timing)

```sql
UPDATE tiers SET hierarchy = 1 WHERE id = 1;  -- base corredor
UPDATE tiers SET payment_required = true, tier_amount = 10000, hierarchy = 2 WHERE id = 4;  -- premium corredor
UPDATE tiers SET hierarchy = 1 WHERE id = 3;  -- base entrenador
UPDATE tiers SET payment_required = true, tier_amount = 100000, hierarchy = 2 WHERE id = 5;  -- medium entrenador
UPDATE tiers SET payment_required = true, tier_amount = 350000, hierarchy = 3 WHERE id = 2;  -- premium entrenador
```

Montos confirmados por el usuario: corredor premium $10.000; entrenador medium $100.000, premium $350.000; ambos roles con base gratis. Correr contra `SUPABASE_TESTING_DATABASE_URL` (y replicar en producción cuando corresponda — no evaluado todavía en ese stage).

## 3. Causa raíz — código (por qué se puede repetir)

`services/tier_service.go` (`Create`/`Update`) **nunca escribe `Hierarchy`** en el modelo — cualquier tier nuevo creado vía `POST /api/v1/tiers` nace con `hierarchy = 0` sin importar el nombre. Tampoco implementa la regla ya definida en el diseño original (`openspec/changes/cambio-tier-suscripciones/specs/tier-subscriptions/spec.md`, requirement *"La gratuidad de un tier se define por payment_required"*): al crear/actualizar un tier llamado `"base"`, el sistema debería forzar `payment_required = false` y asignar `hierarchy` acorde automáticamente — hoy no lo hace, queda librado a que quien lo crea lo setee bien a mano.

**Consecuencia:** el mismo problema (tier pago con `payment_required=false`, o jerarquía sin setear) puede volver a aparecer con cualquier tier nuevo que se cree, no es exclusivo de estos 4.

No se tocó código para este reporte — es una mejora a evaluar aparte, decisión del admin del backend si se prioriza ahora o después.

## 4. Dato inesperado — tier de más en `corredor`

Según la regla de negocio confirmada por el usuario, el rol `corredor` debería tener **solo dos** tiers: base (gratis) y premium ($10.000). Existe una tercera fila, `id=6` "medium corredor" (`created_at` 2026-09-01 — mismo día que se creó "medium entrenador", que sí es legítimo, entrenador tiene base/medium/premium a propósito). Parece un tier creado por error, calcado del de entrenador.

**No se tocó** — decisión pendiente del admin del backend. Antes de borrarlo (soft-delete vía `tierDao.SoftDelete`, mecanismo ya existente) hay que confirmar que ningún registro (`user_role_tier_subscriptions`, `installments`) referencia `tier_id=6` todavía.

## 5. Usuarios con tier pago heredado, sin suscripción que lo respalde

Al pasar `payment_required=true` en "premium entrenador" (id=2), 3 usuarios existentes en testing quedan con acceso premium sin haber pagado nunca — lo tenían asignado de cuando ese tier era gratis:

```sql
SELECT ur.id, ur.user_id, ur.role_id, ur.tier_id, t.name, t.payment_required
FROM user_roles ur JOIN tiers t ON t.id = ur.tier_id
WHERE ur.tier_id IN (2, 4, 5, 6);
```

| user_roles.id | user_id | tier |
|---|---|---|
| 11 | 1 | premium entrenador |
| 12 | 7 | premium entrenador |
| 13 | 10 | premium entrenador |

**No rompe nada hoy** — el acceso lo gobierna `user_roles.tier_id` directamente (decisión de diseño D3 de la spec), no depende de que exista una suscripción vigente. **Decisión (usuario, 2026-09-03): dejarlos como están.** Quedan como casos históricos de antes de que el gate de pago existiera. Revisar si esto cambia si en el futuro alguna feature empieza a chequear historial de pago para algo (no hay ninguna hoy).

## 6. Hallazgo relacionado (mismo tipo de problema, distinto lugar)

No es parte de este reporte pero es la misma categoría — **seed data desalineada con código nuevo**: `POST /api/v1/users/:id/trainer-role` (`services/user_role_service.go`, `defaultTierName = "base"`) busca un tier llamado literalmente `"base"` para el rol entrenador, pero en la DB de testing el tier base de entrenador se llama `"base entrenador"` (mismo patrón de nombres que "medium entrenador"/"premium entrenador"). Activar entrenador falla con 500 para cualquier usuario nuevo en testing hoy. Detectado el 2026-09-03 probando el PR de fotos de perfil, sin relación con la feature de tiers.

## Resumen de qué necesita el admin del backend

1. **Decidir cuándo aplicar** el UPDATE de datos de la sección 2 (SQL ya listo).
2. **Decidir qué hacer con `tier_id=6`** (sección 4) — soft-delete, dejarlo, o borrado duro si nada lo referencia.
3. **Evaluar** si vale la pena implementar la regla D11 (auto-fijar `payment_required`/`hierarchy` para tiers "base") en `tier_service.go`, para que esto no se repita con tiers futuros.
4. **Nada que hacer** con los 3 usuarios de la sección 5 — decisión tomada, dejarlos como están.
5. **Evaluar** el mismatch de nombre de tier en `ActivateEntrenador` (sección 6) — bug separado, mismo tipo de causa raíz.
