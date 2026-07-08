## 1. Constantes de estado

- [x] 1.1 Crear `domains/constants/user_status.go` con tipo `UserStatus string`, constantes exportadas (`UserStatusActive`, `UserStatusInactive`, `UserStatusPause`, `UserStatusBlocked`, `UserStatusSuspended`)
- [x] 1.2 Implementar función `GetValidUserStatuses() []string` que retorne lista completa de estados válidos
- [x] 1.3 Implementar función `IsValidUserStatus(status string) bool` para validación

## 2. Modelo de datos

- [x] 2.1 Agregar campo `Status string gorm:"default:active"` al modelo `dbs.User` en `domains/dbs/user.go`

## 3. DTOs de dominio user

- [x] 3.1 Crear `domains/user/update_request.go` con `UserUpdateRequest` (campos con punteros `*string`, excluye id, status, created_at, updated_at, password)
- [x] 3.2 Crear `domains/user/update_response.go` con `UserUpdateResponse` (todos los campos editables + id + status, excluye password)
- [x] 3.3 Crear `domains/user/status_change_request.go` con `StatusChangeRequest` (campo `Status string binding:"required"`)

## 4. Capa de datos (DAO)

- [x] 4.1 Extender `daos/user_dao.go` con interfaz y métodos: `FindByID`, `FindByEmail`, `Update`, `UpdateStatus`

## 5. Capa de negocio (Service)

- [x] 5.1 Crear `services/user_service.go` con interfaz `UserServiceInterface` e implementación con método `Update(ctx, id, req, currentPassword)` que incluye: validación de email único, autenticación de password para cambio de email, mapeo de campos (solo los no nil), transformación a response
- [x] 5.2 Implementar método `ChangeStatus(ctx, id, status)` que valida estado y actualiza

## 6. Capa de presentación (Controller)

- [x] 6.1 Crear `controllers/user_controller.go` con handler `Update` para `PUT /api/v1/users/:id` (obtener header X-Current-Password, bind JSON, validar formato, delegar al service)
- [x] 6.2 Implementar handler `ChangeStatus` para `PATCH /api/v1/users/:id/status` (bind StatusChangeRequest, validar estado con constantes, delegar al service)

## 7. Routing y wiring

- [x] 7.1 Agregar rutas en `app/url_mappings.go`: `PUT /api/v1/users/:id` y `PATCH /api/v1/users/:id/status`
- [x] 7.2 DI existente compatible — sin cambios necesarios en `app/app.go`

## 8. Modificar registro existente para incluir status

- [x] 8.1 Agregar campo `Status` a `RegisterResponse` en `domains/auth/register_response.go`
- [x] 8.2 Actualizar `services/auth_service.go` para setear `status = constants.UserStatusActive` al crear usuario
- [x] 8.3 Actualizar transformer `toResponse` en auth service para incluir status

## 9. Tests

- [x] 9.1 Escribir tests para `domains/constants/user_status.go` (constantes, validación)
- [x] 9.2 Escribir tests para `services/user_service.go` (Update exitoso, email con password, email duplicado, usuario no encontrado, ChangeStatus con estado válido/inválido)
- [x] 9.3 Escribir tests para `controllers/user_controller.go` (status codes, validaciones, headers)
- [x] 9.4 Escribir tests para `daos/user_dao.go` (integración básica)

## 10. Documentación Swagger

- [x] 10.1 Agregar anotaciones swaggo a los nuevos endpoints (PUT /users/:id, PATCH /users/:id/status)
- [x] 10.2 Documentar header X-Current-Password en el endpoint de actualización
- [x] 10.3 Documentar lista de estados válidos en el endpoint de cambio de estado