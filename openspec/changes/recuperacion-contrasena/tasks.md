## 1. Modelo de datos

- [x] 1.1 Crear `domains/dbs/password_reset_token.go` con struct GORM `PasswordResetToken` (id, user_id, code_hash, expires_at, used_at, attempts, deleted_at, created_at) y `TableName()`, comentarios en español siguiendo el estilo de `user_role.go`
- [x] 1.2 Agregar `&dbs.PasswordResetToken{}` al `AutoMigrate(...)` en `infrastructure/postgresdb/postgres.go`

## 2. DAO

- [x] 2.1 Crear `daos/password_reset_dao.go` con interfaz `PasswordResetDaoInterface` (`Create`, `FindActiveByUserID`, `IncrementAttempts`, `MarkUsed`, `SoftDeleteByUserID`) e implementación, siguiendo el template de `user_role_dao.go`
- [x] 2.2 Crear `daos/password_reset_dao_test.go` con el mismo patrón que el resto de los DAOs del repo (`user_role_dao_test.go`, `user_dao_test.go`): constructor no nulo, implementa la interfaz, no panickea — sin DB real ni SQLite, consistente con la decisión ya tomada de posponer el refuerzo de tests de DAOs con DB real a una rama futura dedicada (no se reintroduce ese scope acá)

## 3. Mailer — email de recuperación

- [x] 3.1 Crear `infrastructure/mailer/templates/reset.html`: mismo esqueleto visual que `welcome.html` (tabla, CSS inline, colores de marca), código de 6 dígitos en fuente grande/destacada (letter-spacing, centrado), texto mencionando expiración de 10 minutos
- [x] 3.2 Agregar struct `PasswordResetEmailData{Name, Code string}` y función `RenderPasswordResetEmail(data) (string, error)` en `infrastructure/mailer/render.go`, con `//go:embed templates/reset.html`
- [x] 3.3 Agregar `SendPasswordResetEmail(ctx, to, name, code string) error` a `MailerInterface` y a `*Client` en `infrastructure/mailer/mailer.go`
- [x] 3.4 Agregar `TestRenderPasswordResetEmail_NoSMTPRequired` en `mailer_test.go` (verifica que el HTML contiene el código y "Paceron"), sin requerir credenciales SMTP

## 4. Servicio

- [x] 4.1 Crear `services/password_reset_service.go` con `PasswordResetServiceInterface` (`RequestPasswordReset(ctx, email) error`, `ResetPassword(ctx, email, code, newPassword) error`), constantes `otpExpiryDuration = 10 * time.Minute`, `otpMaxAttempts = 5`, `otpCodeLength = 6`
- [x] 4.2 Implementar `RequestPasswordReset`: buscar usuario por email (authDao.FindByEmail) — si no existe o no está activo, loguear warning y retornar nil sin enviar mail (nunca error); si existe y está activo: invalidar códigos pendientes previos (`SoftDeleteByUserID`), generar código de 6 dígitos random, hashear con bcrypt, persistir con expiración, enviar mail (best-effort, error de envío se loguea pero no se propaga)
- [x] 4.3 Implementar `ResetPassword`: buscar usuario por email (no encontrado/no activo → error genérico), buscar código activo (`FindActiveByUserID`) (no encontrado/expirado → error genérico), comparar código con bcrypt (no coincide → `IncrementAttempts`, si supera el máximo invalidar el código, error genérico), hashear nueva contraseña con bcrypt, `userDao.Update`, `passwordResetDao.MarkUsed`. La validación de `newPassword == confirmPassword` y `ValidatePassword(newPassword)` se hace en el **controller**, no acá — mismo patrón que `Register` (`services.ValidatePassword` se llama desde `auth_controller.go`, no desde `authService.Register`)
- [x] 4.4 Crear `services/password_reset_service_test.go` cubriendo todos los casos listados en `design.md` (usuario no encontrado, usuario no-activo, invalidación de código previo, código incorrecto incrementa intentos, código expirado, intentos agotados, contraseñas no coinciden, contraseña débil, happy path) usando mocks hand-written (mismo patrón que `auth_service_test.go`)

## 5. Controller

- [x] 5.1 Crear `domains/auth/forgot_password_request.go` (`ForgotPasswordRequest{Email string}`) y `domains/auth/forgot_password_response.go` (`ForgotPasswordResponse{Message string}`)
- [x] 5.2 Crear `domains/auth/reset_password_request.go` (`ResetPasswordRequest{Email, Code, NewPassword, ConfirmPassword string}`) y `domains/auth/reset_password_response.go` (`ResetPasswordResponse{Message string}`)
- [x] 5.3 Crear `controllers/password_reset_controller.go` con `PasswordResetController` (`ForgotPassword`, `ResetPassword`), bind JSON, mapeo de errores: `forgot-password` siempre 200; `reset-password` mapea validación de input (contraseñas no coinciden, contraseña débil) a 400 con mensaje específico, y el error genérico de seguridad a 400 con mensaje único, infra a 500
- [x] 5.4 Crear `controllers/password_reset_controller_test.go` cubriendo: `ForgotPassword` siempre 200 (email válido/inválido/inexistente en el mock), `ResetPassword` happy path 200, contraseñas no coinciden 400, contraseña débil 400, error genérico de seguridad 400, error de infra 500, JSON malformado 400

## 6. Wiring y rutas

- [x] 6.1 Actualizar `app/app.go`: construir `passwordResetDao`, `passwordResetService` (inyectando `authDao`, `userDao`, `passwordResetDao`, el mailer ya existente), `passwordResetController`
- [x] 6.2 Actualizar `app/url_mappings.go`: agregar `POST /api/v1/auth/forgot-password` y `POST /api/v1/auth/reset-password`

## 7. Verificación end-to-end

- [x] 7.1 Ejecutar `go build ./...` y `go vet ./...` — confirmar sin errores
- [x] 7.2 Ejecutar `go test ./...` — confirmar todo verde
- [x] 7.3 Regenerar Swagger (`swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`) con anotaciones para los 2 endpoints nuevos
- [ ] 7.4 Con el backend corriendo y credenciales SMTP reales: pedir reset para un usuario de prueba, confirmar que llega el mail con el código, resetear la contraseña, y loguear exitosamente con la nueva contraseña — pendiente, requiere credenciales reales y servidor corriendo, a cargo del usuario
- [ ] 7.5 Probar manualmente: email inexistente en `forgot-password` responde igual que uno existente; código incorrecto 5 veces invalida el código; código expirado (esperar >10 min o ajustar temporalmente la constante en un entorno de prueba) responde el mismo error genérico — pendiente, idem 7.4
