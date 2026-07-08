## Why

El endpoint de registro de usuarios actual crea usuarios sin un estado explícito, lo que impide gestionar el ciclo de vida de la cuenta (activación, suspensión, bloqueo). Además, no existe forma de modificar los datos de un usuario registrado ni cambiar su estado, funcionalidades básicas para cualquier sistema de gestión de usuarios en producción.

## What Changes

- **Columna `status` en modelo User**: Agregar campo `status` al modelo GORM `dbs.User` con valores: `active`, `inactive`, `pause`, `blocked`, `suspended`. Valor por defecto: `active` al registrarse.
- **Archivo de constantes**: Crear `domains/constants/user_status.go` con constantes tipadas para los estados válidos y documentación.
- **Endpoint PUT /api/v1/users/{id}**: Permitir modificar cualquier atributo del usuario (excepto ID). Para cambiar email, requerir autenticación de password actual por header `X-Current-Password`.
- **Endpoint PATCH /api/v1/users/{id}/status**: Cambiar exclusivamente el estado del usuario, validando que sea un estado válido de la lista de constantes.
- **Validaciones**: Reutilizar validaciones existentes del registro + validaciones específicas de transición de estado.
- **Documentación**: Documentar constantes y endpoints en Swagger.

## Capabilities

### New Capabilities
- `user-status`: Gestión de estados de usuario (constantes, validación, transición)
- `user-update`: Modificación de atributos de usuario registrado
- `user-status-change`: Cambio explícito de estado de usuario

### Modified Capabilities
- `user-registration`: El registro ahora inicializa `status = "active"` por defecto

## Impact

- **Modificar**: `cmd/api/domains/dbs/user.go` — agregar campo `status` al modelo GORM
- **Nuevo**: `cmd/api/domains/constants/user_status.go` — constantes y validación de estados
- **Modificar**: `cmd/api/domains/auth/register_request.go` / `register_response.go` — incluir status en response
- **Modificar**: `cmd/api/services/auth_service.go` — setear status active al registrar
- **Nuevo**: `cmd/api/domains/user/update_request.go` — DTO para PUT /users/{id}
- **Nuevo**: `cmd/api/domains/user/update_response.go` — DTO response para actualización
- **Nuevo**: `cmd/api/domains/user/status_change_request.go` — DTO para PATCH /users/{id}/status
- **Nuevo**: `cmd/api/controllers/user_controller.go` — handlers PUT /users/{id} y PATCH /users/{id}/status
- **Nuevo**: `cmd/api/services/user_service.go` — lógica de actualización y cambio de estado
- **Nuevo**: `cmd/api/daos/user_dao.go` — métodos FindByID, Update, UpdateStatus
- **Modificar**: `cmd/api/app/url_mappings.go` — nuevas rutas
- **Modificar**: `cmd/api/app/app.go` — wiring de nuevos componentes
- **Dependencia**: Ninguna nueva (usa bcrypt, customlogger, apierror existentes)
- **Swagger**: Documentar nuevos endpoints y DTOs