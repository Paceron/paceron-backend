## 1. Template HTML de despedida

- [x] 1.1 Crear `infrastructure/mailer/templates/farewell.html` con el mismo layout de tabla, CSS inline y colores de marca que `welcome.html`, variable `{{.Name}}`
- [x] 1.2 Verificar visualmente el HTML antes de continuar

## 2. Renderizado de templates

- [x] 2.1 Modificar `infrastructure/mailer/render.go`: agregar `//go:embed templates/farewell.html`, struct `FarewellEmailData { Name string }` y función `RenderFarewellEmail(data FarewellEmailData) (string, error)`

## 3. Extender el mailer

- [x] 3.1 Agregar `SendFarewellEmail(ctx, to, name string) error` a `MailerInterface` en `mailer.go`
- [x] 3.2 Implementar `(*Client).SendFarewellEmail` en `mailer.go`, renderizando `farewell.html` y llamando a `Send` con asunto "Tu cuenta fue desactivada"

## 4. Conectar al flujo de baja de usuario

- [x] 4.1 Modificar `services/user_service.go`: agregar campo `mailer mailer.MailerInterface` a `userService`, actualizar `NewUserService` para recibirlo
- [x] 4.2 Modificar `ChangeStatus`: capturar `previousStatus` antes de `UpdateStatus`, invocar `SendFarewellEmail` solo si `previousStatus != "inactive" && status == "inactive"` (nil-checked, error solo logueado, nunca bloquea la respuesta)
- [x] 4.3 Actualizar los 10 call-sites de `NewUserService(mockDao)` en `user_service_test.go` a `NewUserService(mockDao, nil)`
- [x] 4.4 Actualizar los 5 call-sites de `NewUserService(mockDao)` en `opt_service_test.go` a `NewUserService(mockDao, nil)`
- [x] 4.5 Wire en `app.go`: mover el bloque "Mailer" antes del bloque "User flow", pasar `mailerClient` a `services.NewUserService(userDao, mailerClient)`
- [x] 4.6 Ejecutar `go build ./...`, `go vet ./...`, `go test ./...` — todo verde

## 5. Tests dedicados

- [x] 5.1 Agregar `mockMailer` en `opt_service_test.go` (patrón func-field, mismo criterio que `mockUserDao`), implementando `mailer.MailerInterface`
- [x] 5.2 `TestChangeStatus_InactiveSendsFarewellEmail`: active → inactive, asserta que `SendFarewellEmail` fue invocado con email/nombre correctos
- [x] 5.3 `TestChangeStatus_InactiveMailerErrorDoesNotBlock`: mailer retorna error, asserta que `ChangeStatus` igual retorna éxito
- [x] 5.4 `TestChangeStatus_NonInactiveStatusDoesNotSendEmail`: active → pause, asserta que el mailer nunca fue invocado
- [x] 5.5 `TestChangeStatus_RedundantInactiveDoesNotResend`: inactive → inactive, asserta que el mailer nunca fue invocado
- [x] 5.6 `TestChangeStatus_NilMailerDoesNotPanic`: `NewUserService(mockDao, nil)`, active → inactive, asserta que no hay panic y la respuesta es exitosa
- [x] 5.7 `TestRenderFarewellEmail_NoSMTPRequired` en `mailer_test.go`: verifica el renderizado del template sin credenciales SMTP
- [x] 5.8 Ejecutar `go test ./cmd/api/services/... -v -run TestChangeStatus` y `go test ./cmd/api/infrastructure/mailer/... -v -run TestRenderFarewellEmail` — todo verde

## 7. Feedback de code review

- [x] 7.1 Unificar `SendWelcomeEmail`/`SendFarewellEmail` en un único `SendEmail(ctx, to, emailType, data)`, con registro `emailTemplates` (tipo → asunto + template) en `render.go`
- [x] 7.2 Construir el cliente SMTP una sola vez en `mailer.New` y reutilizarlo en cada envío, en lugar de instanciar uno por correo
- [x] 7.3 Ajustar `app.go` para que `mailerClient` quede como interfaz nil si falla la construcción del mailer (evita un `*Client` nulo envuelto en interfaz no-nil)
- [x] 7.4 Actualizar consumidores (`auth_service.go`, `user_service.go`) y `mockMailer` a la nueva interfaz
- [x] 7.5 Ampliar cobertura de `infrastructure/mailer`: render de ambos tipos, tipo desconocido, auto-escaping (requisito de la spec que no tenía test), nombre vacío, registro completo de tipos, construcción del cliente único, puerto por defecto, puerto inválido, reutilización de la instancia SMTP entre envíos
- [x] 7.6 Ampliar cobertura de `Register` + mailer en `auth_service_test.go` (cobertura que había quedado diferida en `agregar-envio-mails-smtp`): envía el correo, error no bloquea, mailer nil no rompe, alta fallida no envía correo
- [x] 7.7 Verificar `go build ./...`, `go vet ./...`, `go test ./...` y el test de integración real con ambos tipos de correo — todo verde

## 6. Verificación end-to-end

- [x] 6.1 Con el backend corriendo y credenciales SMTP reales, registrar un usuario de prueba descartable vía `POST /api/v1/auth/register` (user_id 13)
- [x] 6.2 Invocar `PATCH /api/v1/users/:id/status` con `{"status": "inactive"}` sobre ese usuario
- [x] 6.3 Confirmar en logs la línea `user status changed successfully` seguida de `email enviado exitosamente` para ese `user_id`
- [x] 6.4 Verificar manualmente en la bandeja de entrada del usuario de prueba que el correo llegó, con asunto "Tu cuenta fue desactivada", nombre interpolado y colores de marca visibles
- [x] 6.5 Invocar `PATCH /api/v1/users/:id/status` con `{"status": "inactive"}` una segunda vez sobre el mismo usuario y confirmar en logs que NO se repite la línea de envío de correo (guarda de reenvío funcionando — segunda respuesta en 170ms vs 3.25s de la primera, sin línea de envío de mail)
