## Why

El frontend (Expo managed workflow, Android) no tiene ningún mecanismo para avisarle al usuario eventos que le pasan sin que él mismo lo haya disparado (invitación recibida, alguien respondió su invitación, lo expulsaron de un equipo, un corredor dejó su equipo, cambió su contraseña) más allá de un badge in-app que solo se entera al abrir la app. Coordinado con el frontend vía dos docs (`paceron-frontend/docs/superpowers/specs/2026-08-16-notifications-design.md` y `paceron-frontend/docs/BACKEND_NOTIFICATIONS_REQUIREMENTS.md`): el backend expone registro de device token + el mecanismo de envío, el patrón estándar para Expo managed workflow (push siempre lo dispara un servidor, no hay "directo del front").

Se revisaron los 5 triggers acordados contra el mailer existente (`infrastructure/mailer`, migrado a Resend en la rama anterior) y se confirmó que 4 de los 5 no tenían mail equivalente — se cierra esa brecha sumando también el mail nuevo, en vez de dejar push y mail desalineados.

## What Changes

- **Registro de dispositivo**: `domains/dbs.PushToken` nuevo (tabla `push_tokens`), `daos.PushTokenDaoInterface` (`Upsert` por `token` como clave única — no por `user_id`, así el mismo dispositivo puede cambiar de cuenta sin un endpoint de "desvincular"; `FindByUserID`), `POST /api/v1/push-tokens` (self-only, protegido).
- **Envío**: `restclients/expopushclient` — HTTP plano contra `https://exp.host/--/api/v2/push/send` (sin SDK), mismo patrón que `exampleweatherclient`, reusa `infrastructure/httpclient`.
- **Helper compartido `sendPushToUser`** (función de paquete en `services/`, no un service — `.agentics/CONVENTIONS.md` prohíbe que un service importe a otro; mismo criterio ya usado para `isEntrenadorOfTeam`): busca los tokens del usuario y les manda el push, best-effort.
- **5 triggers cableados** en los mismos puntos donde el flujo principal ya vive:
  - `invitation_service.InviteRunner` → push al invitado (mail ya existía)
  - `invitation_service.AcceptInvitation`/`RejectInvitation` → mail nuevo (`EmailTypeInvitationResponse`) + push al entrenador que invitó
  - `team_user_service.RemoveUser` (rama `callerID != targetUserID`, expulsión) → mail nuevo (`EmailTypeTeamRemoved`) + push al corredor expulsado
  - `team_user_service.RemoveUser` (rama `callerID == targetUserID`, se va solo) → mail nuevo (`EmailTypeTeamMemberLeft`) + push al entrenador
  - `user_service.ChangePassword` → mail nuevo (`EmailTypePasswordChanged`) + push al propio usuario (informativo, sin ruta)
- `team_user_service` gana `mailer.MailerInterface` como dependencia nueva (antes no mandaba mail).
- Best-effort en los 4 triggers nuevos: un fallo de mail o push se loguea y nunca bloquea la operación principal — mismo criterio que el mail existente en `AcceptInvitation`/`ChangeStatus`/etc. `InviteRunner` mantiene su comportamiento previo (mail bloqueante) sin cambios, fuera de alcance tocarlo acá.

## Capabilities

### New Capabilities
- `push-notifications`: registro de device token y envío de notificaciones push vía Expo para los 5 triggers listados.

## Impact

- **Nuevo**: `domains/dbs/push_token.go`, `domains/constants/push_platform.go`, `daos/push_token_dao.go`, `restclients/expopushclient/`, `domains/pushtoken/`, `services/push_token_service.go`, `services/push_notifier.go`, `controllers/push_token_controller.go`.
- **Modificado**: `infrastructure/mailer/render.go` (4 `EmailType`/templates nuevos), `services/{invitation_service.go,team_user_service.go,user_service.go}` (constructores ganan `pushTokenDao`/`pushClient`; `team_user_service` gana también `mailer`), `app.go` (wiring), `url_mappings.go` (`POST /api/v1/push-tokens`), `infrastructure/postgresdb/postgres.go` (AutoMigrate).
- **Tests**: DAO con Postgres real (`testutils.SetupTestDB`), restclient con `httptest.Server`, tests dirigidos por trigger en los 3 services tocados (notificación exitosa + best-effort ante fallo).
- **Swagger**: regenerado, endpoint nuevo documentado.
- **Docs**: `README.md`, `docs/AUTH_MIGRATION.md` (sección 9).
- **Fuera de alcance**: web push (Push API navegador + Service Worker + VAPID, pila distinta), iOS, triggers de pago (reservados, A/B no existen todavía).
