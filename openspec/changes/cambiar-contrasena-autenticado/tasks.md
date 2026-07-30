## 1. Modelo de datos

- [x] 1.1 Agregar `PasswordChangedAt *time.Time` (`gorm:"column:password_changed_at"`) a `domains/dbs/user.go` — nullable, aditivo, sin migración manual (AutoMigrate)

## 2. DTOs

- [x] 2.1 Crear `domains/user/change_password_request.go` (`ChangePasswordRequest{CurrentPassword, NewPassword, ConfirmPassword string}`, todos `binding:"required"`)
- [x] 2.2 Crear `domains/user/change_password_response.go` (`ChangePasswordResponse{Message string}`)

## 3. Servicio

- [x] 3.1 Agregar `ChangePassword(ctx, id int64, currentPassword, newPassword string) error` a `UserServiceInterface` y su implementación en `services/user_service.go`: buscar user (nil → "usuario no encontrado"), `bcrypt.CompareHashAndPassword` contra el hash actual (mismatch → "contraseña actual incorrecta"), rechazar si `newPassword` coincide con el hash actual ("la nueva contraseña debe ser distinta a la actual"), hashear con bcrypt, `userDao.Update` seteando `Password` y `PasswordChangedAt = &now`
- [x] 3.2 Tests en `services/user_service_test.go`: éxito, usuario no encontrado, contraseña actual incorrecta, nueva contraseña igual a la actual, error de DAO en `Update`

## 4. Controller

- [x] 4.1 Agregar `ChangePassword(c *gin.Context)` a `UserController`/`userController` en `controllers/user_controller.go`: parsea `:id`, bind body, valida `new_password == confirm_password` (400 si no) y `services.ValidatePassword(new_password)` (400 si falla), llama al service, mapea errores (`usuario no encontrado`→404, `contraseña actual incorrecta`→401, `la nueva contraseña debe ser distinta a la actual`→400, resto→500)
- [x] 4.2 Tests en `controllers/user_controller_test.go`: happy path, mismatch de confirmación, contraseña débil, contraseña actual incorrecta, usuario no encontrado, nueva igual a la actual, JSON malformado

## 5. Backfill en el flujo de reset existente

- [x] 5.1 En `services/password_reset_service.go`, método `ResetPassword`: setear `userDB.PasswordChangedAt = &now` antes de `userDao.Update`
- [x] 5.2 Test en `services/password_reset_service_test.go` confirmando que se setea en el happy path

## 6. Rutas y documentación

- [x] 6.1 Agregar `PATCH /api/v1/users/:id/password` en `app/url_mappings.go`
- [x] 6.2 Regenerar Swagger, actualizar tabla de endpoints en `README.md`
- [x] 6.3 Agregar request a las colecciones Bruno (`change password v1`, ambas carpetas)

## 7. Verificación

- [x] 7.1 `go build ./...`, `go vet ./...`, `go test ./...` — todo verde
- [ ] 7.2 Probar manualmente: cambiar contraseña de un usuario de prueba, confirmar que loguea con la nueva y falla con la vieja — pendiente, requiere servidor corriendo, a cargo del usuario
