## Context

`InviteRunner` existía desde antes (envía un email vía `mailer.SendEmail(..., mailer.EmailTypeInvitation, ...)`) pero no persistía nada — no hay modelo `Invitation` en `domains/dbs/`. El template de email (`infrastructure/mailer/templates/invitation.html`) ya asume un flujo in-app ("ingresá a la aplicación y aceptá la notificación pendiente"), pero esa notificación no existía. `TeamUser` (membresía equipo-usuario) y `PasswordResetToken` (modelo de referencia con estado/expiración/soft-delete) ya existen y se reusan como patrón.

## Goals / Non-Goals

**Goals:**
- Persistir la invitación al momento de invitar, sin cambiar el contrato HTTP existente de `POST /api/v1/teams/:id/invite`.
- Permitir listar pendientes por equipo y aceptar/rechazar por el propio invitado.
- Evitar invitaciones duplicadas (misma persona, mismo equipo, ya pendiente).

**Non-Goals:**
- Token secreto/magic-link por email — la identidad del que responde se resuelve por `user_id` (mismo patrón que el resto de la app, sin middleware de auth), no por posesión de un secreto.
- Job de limpieza de invitaciones vencidas — el chequeo de expiración es perezoso (en el momento de listar/responder), igual que `password_reset_service` hace con `time.Now().After(tokenDB.ExpiresAt)`.

## Decisions

### 1. `Invitation` sí lleva expiración, pero no es un mecanismo de seguridad
**Por qué**: a diferencia de `PasswordResetToken` (protege contra fuerza bruta de un código), acá no hay secreto que expire por seguridad. Pero una invitación "viva" para siempre es un problema de negocio real (el equipo pudo cambiar, el invitado se olvida y aparece meses después). `ExpiresAt` (15 días) es informativo: se chequea en el momento de listar/responder, sin cron.
**Alternativa descartada**: sin expiración — se descarta porque dejaría invitaciones pendientes indefinidamente sin ninguna forma de que naturalmente dejen de ser válidas.

### 2. Recurso plano `/api/v1/invitations/:id/accept|reject`, no anidado bajo `/teams/:id/invitations/:invitation_id/...`
**Por qué**: el actor de accept/reject es el propio invitado respondiendo por sí mismo — el `team_id` es redundante una vez que se tiene el `invitation_id` (se resuelve internamente vía `inv.TeamID`). El listado (`GET /api/v1/teams/:id/invitations`) sí queda bajo el team porque ahí el equipo es el sujeto de la consulta (mismo patrón que `GET /api/v1/teams/:id/users`).
**Alternativa descartada**: anidar accept/reject bajo `/teams/:id/invitations/:invitation_id/...` — agrega una validación extra sin beneficio real (¿qué pasa si `team_id` no matchea? sería un chequeo artificial).

### 3. `InviterID` = `team.OwnerID`, sin tocar el contrato de `InviteRunnerRequest`
**Decisión del usuario** (2026-07-29): agregar `inviter_user_id` obligatorio al request rompería el contrato ya consumido por el frontend. Se resuelve como `team.OwnerID` en el service.
**Trade-off aceptado**: si en el futuro un equipo tiene co-entrenadores que invitan, quedaría mal atribuido el owner como inviter — se revisita si ese escenario aparece.

### 4. Orden de operaciones en `AcceptInvitation`: crear `TeamUser` ANTES de marcar la invitación como aceptada
**Por qué**: el repo no usa transacciones GORM en ningún flujo similar (`AddUser`, `RequestPasswordReset` tampoco). Si el segundo paso (marcar aceptada) falla después de crear el `TeamUser`, un reintento del propio usuario detecta que ya es miembro (`teamUserDao.FindByTeamAndUser`) y solo corrige el estado de la invitación sin duplicar el alta — autocorrección de bajo costo. El orden inverso (marcar aceptada primero) dejaría al usuario con la invitación "resuelta" pero sin acceso real al equipo, sin ninguna vía de reintento (el chequeo de estado ya no permitiría re-procesar).
**Por qué no una transacción real**: sin precedente en el repo, ambos pasos son inserts/updates simples sin llamadas externas entre medio — bajo riesgo e impacto acotado, se prioriza consistencia con el resto del código.

### 5. Duplicados se previenen a nivel aplicación, no con constraint único en DB
**Por qué**: igual que `TeamUser` (no tiene unique(team_id, user_id) a nivel DB, se chequea vía `FindByTeamAndUser`), se mantiene consistencia con el resto del repo — ningún otro modelo define constraints compuestos en `AutoMigrate`.

## Risks / Trade-offs

- `ListPendingInvitations` hace una query adicional por invitación para resolver nombre/email del invitado (`userDao.FindByID`) — aceptable hoy (equipos no manejan volúmenes altos de invitaciones simultáneas simultáneas); si se vuelve un problema de performance, se resuelve con un `FindByIDs` batch en `userDao`.
- Sin transacción real en `AcceptInvitation` (ver Decisión 4) — mitigado por el orden de operaciones, no por atomicidad real.
