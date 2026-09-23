# Deuda técnica y decisiones pendientes conocidas

Conocimiento acumulado en sesiones previas de asistente de IA (memoria privada de esas sesiones, no reconstruible por otra herramienta) que nunca llegó a quedar escrito en el repo. Volcado acá el 2026-09-19 al migrar el desarrollo de este repo a OpenCode, para que no se pierda. Actualizar esta lista a medida que cada ítem se resuelve o se descarta — no es un backlog formal, es memoria institucional.

## Bugs conocidos, no arreglados

### `ActivateEntrenador` falla 500 en testing (nombre de tier)

`POST /api/v1/users/:id/trainer-role` (`services/user_role_service.go`, `defaultTierName = "base"`) busca un tier llamado exactamente `"base"` para el rol `entrenador` vía `tierDao.FindByNameAndRole`. En la DB de testing, los tiers de `entrenador` están nombrados `"base entrenador"`/`"medium entrenador"`/`"premium entrenador"` — no `"base"` a secas. Cualquier usuario nuevo que intente activar entrenador en testing hoy falla con 500. No verificado en producción. No es un bug introducido por ningún PR reciente, el código no se tocó.

Dos alternativas, a decidir:
1. Corregir el seed de testing para que el tier base de entrenador se llame `"base"`.
2. Cambiar la búsqueda en `user_role_service.go` para matchear por `hierarchy`/`payment_required=false` en vez de nombre literal — más robusto, pero es cambio de código. Conecta con `openspec/changes/cambio-tier-suscripciones/` (usa `hierarchy` como fuente de verdad) — si esa spec avanza, este bug se resuelve como efecto colateral.

### `HandleWebhook` de Mercado Pago no valida la firma HMAC

`mercadopagoclient.ValidateWebhookSignature` existe (wrapea el SDK de MP, HMAC-SHA256) y `payment_service.go` carga `webhookSecret`, pero `HandleWebhook` nunca la llama — el controller solo chequea que `X-Signature`/`X-Request-Id` estén presentes, no que la firma sea válida.

Riesgo acotado: el webhook siempre re-consulta el estado real del pago vía `GET /v1/payments/:id` a la API de MP por ID, así que un atacante no puede forjar un "approved" falso, solo disparar un re-fetch de un pago real conocido. Deferido a propósito por decisión del usuario mientras un compañero trabajaba activamente en pagos/tiers — **antes de arreglarlo, revisar si ese trabajo ya lo cubrió** (`grep HandleWebhook\|ValidateWebhookSignature` en `payment_service.go`). Si sigue faltando, el fix es chico: llamar `mpClient.ValidateWebhookSignature(xSignature, xRequestID, notification.Data.ID, s.webhookSecret)` en `HandleWebhook` antes de procesar.

## Datos desactualizados (testing)

### `tiers`: `hierarchy`/`payment_required`/`tier_amount` sin backfill

Las filas de tiers no-base de la DB de testing fueron creadas antes de que `payment_required`/`tier_amount`/`hierarchy` importaran (feature de pagos llegó después) — nadie las backfilleó. `hierarchy` está en `0` en las 6 filas (debería ser base=1 < medium=2 < premium=3 por rol).

Fix acordado, no aplicado (esperando luz verde explícita del usuario para correrlo):

```sql
UPDATE tiers SET hierarchy = 1 WHERE id = 1;  -- base corredor (payment_required/tier_amount ya correctos)
UPDATE tiers SET payment_required = true, tier_amount = 10000, hierarchy = 2 WHERE id = 4;  -- premium corredor
UPDATE tiers SET hierarchy = 1 WHERE id = 3;  -- base entrenador
UPDATE tiers SET payment_required = true, tier_amount = 100000, hierarchy = 2 WHERE id = 5;  -- medium entrenador
UPDATE tiers SET payment_required = true, tier_amount = 350000, hierarchy = 3 WHERE id = 2;  -- premium entrenador
-- id = 6 (medium corredor) NO tocar — ver ítem siguiente
```

**Gap de código relacionado** (no cubierto por este fix de datos): `tier_service.go` `Create`/`Update` nunca escriben `Hierarchy` (siempre default 0) y no fuerzan `payment_required=false` para tiers llamados "base". Cualquier tier creado hoy vía API repite el mismo problema.

### `tiers.id=6` ("medium corredor") no debería existir

Regla de negocio del usuario: el rol `corredor` solo debe tener 2 tiers (Base gratis, Premium $10000) — `entrenador` sí tiene 3 (base/medium/premium) a propósito. La fila `id=6` fue agregada por error junto con el medium de entrenador (legítimo). **No tocar sin confirmación fresca** — antes de borrar (aunque sea soft-delete), confirmar que ninguna `subscription`/`installment` ya apunta a `tier_id=6`. Pendiente comunicárselo a quien administra los tiers.

## Features deferidas, con diseño ya charlado

### Aceptar invitación por botón en el email (magic link)

Hoy solo se acepta/rechaza in-app (`POST /api/v1/invitations/:id/accept|reject` con `user_id`, JWT de sesión normal) — explícitamente fuera de alcance en `openspec/changes/persistir-invitaciones-aceptar-rechazar/design.md`. Decisión: cuando se aborde, un botón en el email que acepte vía un **token de un solo uso, scopeado únicamente a aceptar esa invitación puntual** (no el JWT de sesión regular) — mismo patrón que `password_reset_token` pero como link en vez de código tipeado. Agregar un endpoint no autenticado `GET/POST /api/v1/invitations/:id/accept?token=...` que valide el token en vez de confiar en `user_id`, expira igual que la invitación. Los endpoints in-app existentes quedan sin cambios — esto es un camino adicional, no un reemplazo.

### Rol "admin" global (post-MVP)

Planeado para después del MVP: moderación a nivel plataforma (banear usuarios, soporte) — distinto del rol `entrenador` (rol de negocio, no de plataforma). Por eso `/api/v1/roles`, `/api/v1/tiers`, `/api/v1/permissions`, `/api/v1/auth/permissions` hoy solo exigen login, sin role-gating (deuda documentada en `docs/AUTH_MIGRATION.md` §6). No inventar un check de admin ad-hoc para tapar ese gap — cuando se aborde, es su propio change de OpenSpec (qué puede hacer, cómo se otorga, audit trail). No tiene relación con la auto-activación de entrenador (`POST/DELETE /api/v1/users/:id/entrenador-role`), que es una transacción del propio usuario sobre su cuenta, no una acción de moderación sobre la cuenta de otro.

### Búsqueda de equipos + solicitudes de ingreso (join requests)

Spec de frontend: `paceron-frontend/docs/superpowers/specs/2026-09-03-team-search-join-requests-design.md` (aprobada del lado frontend, contrato de backend todavía **no** confirmado — esa sección es una propuesta). Frontend construye contra mocks mientras tanto, no está bloqueado.

Deferido explícitamente para evitar colisión con el branch en paralelo `feature/suscripciones-tier-equipos` (team-split payments) — ambas features tocan el mismo punto exacto: la creación de `TeamUser` necesita pasar por un único lugar, porque ahí es donde aplica el gate de cuota de membresía. Ese branch ya extrajo `ApplyTeamMembershipGate(ctx, db, teamUserDao, installDao, teamUser, membershipFee)`, llamado desde `AddUser` y `AcceptInvitation`.

**Decisiones ya confirmadas, siguen válidas al retomar:**
1. `visible`/`is_public` en `Team`: no endpoint nuevo, extender `UpdateTeamRequest` (`Visible *bool`/`IsPublic *bool`, mismo patrón que `ShowGroupsToRunners`), servido por el `PUT /api/v1/teams/:id` ya existente.
2. Badge de pendientes: endpoint dedicado `GET /api/v1/join-requests/pending-count` (COUNT agregado sobre todos los equipos del caller), simétrico a `myInvitationsCount`.
3. Grupo default al aceptar: mismo fallback que `AcceptInvitation` (`Group.IsMain` si no se elige uno).

**Cómo retomar:** confirmar que `feature/suscripciones-tier-equipos` ya mergeó (`git log develop`), verificar que `ApplyTeamMembershipGate` (o su nombre post-merge) sigue con forma similar, diseñar `AcceptJoinRequest` para llamarlo directo desde el día uno (sin refactor previo). Diseño se cortó en brainstorming, antes de secciones formales — todavía pendiente: un change o dos (search vs. join-requests), códigos de error, forma de paginación (sin precedente en el repo hoy), plan de tests.

### Race condition de cupo de equipo (`MaxMembers`)

Check-then-act sin lock, en 3 call sites: `team_user_service.go:AddUser`, `invitation_service.go:AcceptInvitation`, y el futuro `join_request_service.go:Accept`. Dos altas casi simultáneas para el mismo equipo pueden pasar el check ambas y pasarse del cupo por 1.

Decisión: no parchear un solo call site (evaluado `SELECT ... FOR UPDATE` solo para `join-requests`, descartado por inconsistente). El efecto de la carrera es auto-corregible (el entrenador saca al corredor de más manualmente, sin corrupción de datos/dinero) — se acepta el riesgo hasta poder arreglar los 3 call sites juntos, misma estrategia de lock (`SELECT FOR UPDATE` sobre la fila de equipo dentro de una transacción es la más probable), no uno por uno.

## Pendientes de limpieza

### Archivo trackeado por error: `.superpowers/sdd/tasks/task-3-report.md`

Sobrevive en `develop` trackeado en git (entró con commits de Task 3 del change `asignacion-por-instanciacion`, `df18d60`/`2babf15`, por un `git add` amplio del implementador que forzó/bypaseó el ignore del workspace SDD). El workspace `.superpowers/sdd/` es scratch auto-ignoreado (su propio `.gitignore` con `*`) — briefs/reports/ledger de `subagent-driven-development` son artefactos de recuperación intra-sesión, sin valor en el historial. Fix: `git rm .superpowers/sdd/tasks/task-3-report.md`, aprovechado cualquier rama futura (no amerita PR propio). Prevención: en dispatchs de implementadores exigir stageear rutas explícitas por commit, nunca `git add -A`/`git add .` en la raíz del repo ni force-add de rutas ignoradas (registrado también en `AGENTS.md` §9).

## Cobertura de tests

> Registrado el 2026-09-22 con el change `cobertura-calendario-recientes` (2026-09-22, ronda de tests de calendario/controller/session/exercise/DAOs). Estado luego del change: analyzer **85.2% (7649/8976)**, meta puntual ≥84% superada, gate de `make coverage` subido a 85% (2026-09-23, `chore/coverage-threshold-85`).

### Etapa 2 de coverage: legacy → 90% (trabajo futuro / repuesto)

La meta de esta etapa cubrió calendario/recientes; el resto del repo (paquetes legacy de pagos/usuarios sin calendar) quedó para una **etapa 2** con objetivo 90% total, a usar como trabajo de repuesto cuando no haya gaps ni features nuevas para cubrir. Gaps principales conocidos:

- `payment_service` (~102 ramas), `customlogger` (~78), `mercadopagoclient` (~68, con `httptest` para los paths HTTP reales)
- `user_service` (~54), `tier_service`, `training_plan_service`, `join_request_service`, `workout_feedback_service`
- `team_dao.SearchPublic` (~5, ver parked más abajo)

### Riesgo de acoplamiento del helper `FailingDB` (declarado en design D1)

`cmd/api/testutils/failing_db.go` inyecta fallas discriminando por operación (`nthFail`), lo que acopla los tests al **orden de statements** que gorm ejecuta por query. Un refactor o upgrade de gorm que reordente/autoenvuelva statements rompe estos tests ruidosamente, incluso sin bug real — si pasa, re-ajustar los umbrales/posición de `Nth` de los tests, no "arreglar" el servicio. Ver `openspec/changes/cobertura-calendario-recientes/design.md` D1.

### Ramas parked legítimas (inalcanzables desde fuera, no forzar)

- `calendar_service` bulk con kind `cancelled`: la rama existe pero `BulkRequest` no expone `cancelled_reason`, inalcanzable vía API — se cubrirá junto a la feature real de bulk-cancel.
- Ramas defensivas de `calendar_service` (TOCTOU re-check de "día cerrado", JSON imposible del body): cubren condiciones de carrera/payloads que el handler previo ya filtra; forzarlas en test testearía el test.
- `team_dao.SearchPublic`: se implementa junto a la feature de búsqueda de equipos (deferida, ver arriba).

## Decisiones de "no tocar"

- **`utils.StringToInt64`/`Int64ToString`/`Contains`/`IsPositiveInteger`/`ParseInt64`** (`cmd/api/utils/`): cero call sites en todo el repo, confirmado por grep + `git log --diff-filter=A` (vienen del scaffold inicial). Decisión explícita del usuario: **no borrar**, se guardan para trabajo futuro de métricas/pagos que probablemente los necesite. No re-flaguearlos como dead code en una futura limpieza de coverage.
