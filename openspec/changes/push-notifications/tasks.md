## 1. Registro de dispositivo

- [x] 1.1 `domains/dbs/push_token.go`: `PushToken{ID, UserID, Token (uniqueIndex), Platform, CreatedAt, UpdatedAt}`
- [x] 1.2 `domains/constants/push_platform.go`: `PushPlatform` (`android`/`web`), `IsValidPushPlatform`
- [x] 1.3 Sumar `&dbs.PushToken{}` a `postgresdb.ConfigDB`'s `AutoMigrate`
- [x] 1.4 `daos/push_token_dao.go`: `Upsert` (clave de conflicto `token`, vía `clause.OnConflict`), `FindByUserID`
- [x] 1.5 `domains/pushtoken/{push_token_request.go,push_token_response.go}`
- [x] 1.6 `services/push_token_service.go`: valida platform, delega a `pushTokenDao.Upsert`
- [x] 1.7 `controllers/push_token_controller.go` + ruta `POST /api/v1/push-tokens` (self-only, `user_id` del token de sesión)

## 2. Envío

- [x] 2.1 `restclients/expopushclient/client.go`: `Send(ctx, token, title, body, data)`, HTTP plano contra `https://exp.host/--/api/v2/push/send`
- [x] 2.2 `services/push_notifier.go`: `sendPushToUser` (función de paquete, no service — busca tokens del usuario, manda a cada uno, best-effort)

## 3. Templates de mail nuevos

- [x] 3.1 `infrastructure/mailer/render.go`: `EmailTypeInvitationResponse`, `EmailTypeTeamRemoved`, `EmailTypeTeamMemberLeft`, `EmailTypePasswordChanged` + `EmailData.RelatedUserName`/`ResponseStatus`
- [x] 3.2 4 templates HTML nuevos (`templates/{invitation_response,team_removed,team_member_left,password_changed}.html`), mismo patrón que los 4 existentes (logo embebido, auto-escaping)

## 4. Wiring de triggers

- [x] 4.1 `invitation_service.InviteRunner`: push al invitado tras enviar la invitación (mail ya existía)
- [x] 4.2 `invitation_service.AcceptInvitation`/`RejectInvitation`: `notifyInvitationResponse` — mail + push al entrenador (`inv.InviterID`)
- [x] 4.3 `team_user_service`: gana `mailer.MailerInterface` + `pushTokenDao` + `pushClient` en el constructor (antes no mandaba mail)
- [x] 4.4 `team_user_service.RemoveUser`: `notifyTeamRemoval` (expulsión, rama `callerID != targetUserID`) y `notifyTeamMemberLeft` (se va solo, rama `callerID == targetUserID`)
- [x] 4.5 `user_service.ChangePassword`: mail + push al propio usuario, sin ruta (informativo)
- [x] 4.6 Todos los triggers nuevos son best-effort (nil-check + log, nunca bloquean la operación principal) — `InviteRunner` mantiene su comportamiento previo (mail bloqueante) sin cambios

## 5. Wiring de aplicación

- [x] 5.1 `app.go`: `pushTokenDao` nuevo, `expoPushClient` nuevo (`httpclient.New` con `WithBaseURL("https://exp.host")`, timeout 8s, retry 2), pasado a los 3 services + `push_token_service`/`push_token_controller`
- [x] 5.2 `url_mappings.go`: `POST /api/v1/push-tokens` (ruta protegida)

## 6. Tests

- [x] 6.1 `push_token_dao_test.go`: contra Postgres real (`testutils.SetupTestDB`) — create, reasignación de dueño por upsert, cambio de platform, múltiples dispositivos por usuario
- [x] 6.2 `expopushclient/client_test.go`: contra `httptest.Server` real, verifica shape del request (no mockea el httpClient)
- [x] 6.3 `push_token_service_test.go`/`push_token_controller_test.go`: éxito, platform inválida, error de DAO
- [x] 6.4 Mocks compartidos nuevos en `services`: `mockPushTokenDao` (safe defaults), `mockExpoPushClient` (safe defaults)
- [x] 6.5 Tests dirigidos por trigger: notificación exitosa (mail + push con datos correctos) y fallo de notificación no bloquea la operación principal, en los 3 services tocados
- [x] 6.6 Todos los call sites existentes de `NewInvitationService`/`NewTeamUserService`/`NewUserService` actualizados a las firmas nuevas
- [x] 6.7 `go build ./...` / `go vet ./...` / `go test ./...` verdes, coverage total 86.9% (gate 80%)

## 7. Docs

- [x] 7.1 Swagger regenerado
- [x] 7.2 `README.md`: fila de `POST /api/v1/push-tokens`
- [x] 7.3 `docs/AUTH_MIGRATION.md`: sección 9, triggers y contrato del endpoint

## 8. Verificación manual (a cargo del usuario)

- [ ] 8.1 Registrar un token de prueba vía `POST /api/v1/push-tokens` con un access token real
- [ ] 8.2 Disparar cada uno de los 5 triggers end-to-end y confirmar que el mail nuevo llega y el push se dispara (o inspeccionar el request/response de Expo si no hay device real a mano)
- [ ] 8.3 Confirmar que un fallo simulado de Expo o Resend no rompe el flujo principal
