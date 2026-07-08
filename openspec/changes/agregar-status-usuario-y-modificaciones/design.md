## Context

El proyecto actualmente cuenta con endpoint `POST /api/v1/auth/register` que crea usuarios sin campo `status`. No existen endpoints para modificar usuarios ni gestionar su estado. Se requiere:
1. Agregar columna `status` al modelo User con constantes tipadas (active, inactive, pause, blocked, suspended)
2. Endpoint `PUT /api/v1/users/{id}` para modificar atributos (excepto ID, email requiere password)
3. Endpoint `PATCH /api/v1/users/{id}/status` para cambiar estado con validación

Se reutiliza la infraestructura existente: GORM, customlogger, apierror, bcrypt, gin.

## Goals / Non-Goals

**Goals:**
- Agregar campo `status` al modelo `dbs.User` GORM con default `active`
- Crear paquete `domains/constants/user_status.go` con constantes tipadas y función `IsValidUserStatus()`
- `RegisterResponse` incluir `status` y el registro setear `active` por defecto
- Endpoint `PUT /api/v1/users/{id}` para modificar atributos del usuario
- Endpoint `PATCH /api/v1/users/{id}/status` para cambiar estado
- Validar cambio de email con password actual (header `X-Current-Password`)
- Validar estados contra constantes tipadas
- JSON under_score, Go camelCase, logging con customlogger, errores con apierror.APIError

**Non-Goals:**
- Autenticación/autorización (JWT, sesiones, roles)
- Rate limiting
- Historial de cambios de estado (auditoría)
- Endpoints de listado o búsqueda de usuarios
- Notificaciones de cambio de estado
- Eliminación de usuarios (DELETE)
- Refactor de endpoints existentes de auth

## Decisions

### 1. Paquete de constantes en `domains/constants/`
- **Qué**: Nuevo paquete `domains/constants/user_status.go` con tipo `UserStatus string`, constantes exportadas y funciones `GetValidUserStatuses()` e `IsValidUserStatus()`
- **Por qué**: Las constantes son compartidas entre capas (DAO, Service, Controller). Un paquete separado evita dependencias circulares y centraliza la definición.
- **Alternativa**: Definir en `domains/dbs/user.go` — mezcla responsabilidades de modelo DB con constantes de dominio.

### 2. PUT vs PATCH para modificación de usuario
- **Qué**: Se usa `PUT /api/v1/users/{id}` para modificar atributos y `PATCH /api/v1/users/{id}/status` para cambio de estado
- **Por qué**: PUT recibe el objeto completo a actualizar (reemplazo parcial de campos específicos). PATCH es semánticamente más preciso para un cambio de estado puntual.
- **Implementación**: El PUT acepta todos los campos permitidos (excluyendo id, created_at, status). Los campos no enviados se ignoran (no se sobreescriben con cero). Se usan punteros en el DTO para detectar campos presentes.

### 3. Autenticación de email con header `X-Current-Password`
- **Qué**: Al cambiar el campo `email`, el service valida el password actual recibido por header `X-Current-Password` usando `bcrypt.CompareHashAndPassword`
- **Por qué**: Misma lógica que el registro — el password nunca viaja en body JSON. Previene cambios no autorizados de email.
- **Flujo**: Controller obtiene header → lo pasa al service → service compara con hash almacenado → si no coincide, 401.

### 4. Validación de estados contra constantes
- **Qué**: El controller y el service usan `constants.IsValidUserStatus(status)` para validar que el estado recibido sea válido
- **Por qué**: Centraliza la lógica de validación. Si se agregan/quitan estados en el futuro, solo se modifica el archivo de constantes.
- **Implementación**: El controller valida formato, el service valida lógica de negocio (transiciones permitidas, si aplica).

### 5. UserUpdateRequest con punteros para campos opcionales
- **Qué**: Los campos del DTO `UserUpdateRequest` se declaran con tipos puntero (`*string`) para distinguir entre "no enviado" (nil) y "enviado vacío" (string vacío)
- **Por qué**: En Go, un string vacío es el zero value. Sin punteros no se puede diferenciar entre "no quiero cambiar este campo" y "quiero dejarlo vacío".
- **Alternativa**: Usar `binding` tags y campos directos — no permite detección de campos omitidos.

### 6. User DAO separado de Auth DAO
- **Qué**: Nuevo `daos/user_dao.go` con interfaz `UserDaoInterface` e implementación con métodos `FindByID`, `FindByEmail`, `Update`, `UpdateStatus`
- **Por qué**: Auth DAO se enfoca en registro (Create, FindByEmail, FindByDNI). User DAO se enfoca en gestión de cuentas existentes (FindByID, Update, UpdateStatus). Separar evita que un DAO crezca con responsabilidades mezcladas.
- **Alternativa**: Extender `auth_dao.go` — terminaría siendo un DAO de "todo usuario", violando SRP.

### 7. User Service separado de Auth Service
- **Qué**: Nuevo `services/user_service.go` con interfaz `UserServiceInterface` e implementación con métodos `Update` y `ChangeStatus`
- **Por qué**: La lógica de actualización y cambio de estado es distinta a la lógica de registro. Separar mantiene cohesión y facilita testing.
- **Alternativa**: Agregar métodos a `auth_service.go` — mezcla registro con gestión de cuentas.

### 8. Update no permite cambiar status
- **Qué**: El endpoint `PUT /api/v1/users/{id}` ignora el campo `status` si se envía en el body
- **Por qué**: El status tiene su propio endpoint dedicado (`PATCH .../status`) con validación específica. Mantiene separación de responsabilidades.
- **Implementación**: El campo `status` existe en el modelo pero no se incluye en `UserUpdateRequest`. Si se envía, se ignora silenciosamente.

## Architecture Flow

```
PUT /api/v1/users/:id
  │
  ├─► userController.Update(c *gin.Context)
  │     ├─ Obtener id de la URL (c.Param("id"))
  │     ├─ Obtener X-Current-Password del header (si se envía)
  │     ├─ BindJSON → user.UserUpdateRequest
  │     ├─ Si email presente en request:
  │     │     ├─ Validar formato email
  │     │     └─ Verificar que X-Current-Password fue enviado
  │     ├─ Llamar userService.Update(ctx, id, req, currentPassword)
  │     └─ Responder 200 con user.UserUpdateResponse
  │
  ├─► userService.Update(ctx, id, req, currentPassword) → (*user.UserUpdateResponse, error)
  │     ├─ userDao.FindByID(ctx, id) → obtener usuario actual
  │     ├─ Si email presente y diferente del actual:
  │     │     ├─ Validar password con bcrypt (vs hash almacenado)
  │     │     └─ userDao.FindByEmail(ctx, newEmail) → verificar unicidad
  │     ├─ Mapear campos del request al modelo (solo campos no nil)
  │     ├─ userDao.Update(ctx, userDB) → persistir cambios
  │     ├─ Transformar dbs.User → user.UserUpdateResponse
  │     └─ Log con customlogger.Info
  │
  └─► userDao (implements UserDaoInterface)
        ├─ FindByID(ctx, id) → (*dbs.User, error)
        ├─ FindByEmail(ctx, email) → (*dbs.User, error)
        ├─ Update(ctx, user *dbs.User) → error
        └─ UpdateStatus(ctx, id, status) → error

PATCH /api/v1/users/:id/status
  │
  ├─► userController.ChangeStatus(c *gin.Context)
  │     ├─ Obtener id de la URL
  │     ├─ BindJSON → user.StatusChangeRequest
  │     ├─ Validar status con constants.IsValidUserStatus()
  │     ├─ Llamar userService.ChangeStatus(ctx, id, status)
  │     └─ Responder 200 con user.UserUpdateResponse
  │
  ├─► userService.ChangeStatus(ctx, id, status) → (*user.UserUpdateResponse, error)
  │     ├─ userDao.FindByID(ctx, id)
  │     ├─ userDao.UpdateStatus(ctx, id, status)
  │     ├─ Transformar a user.UserUpdateResponse
  │     └─ Log con customlogger.Info
  │
  └─► userDao (misma implementación anterior)
```

## New files structure

```
cmd/api/
├── domains/
│   ├── constants/
│   │   └── user_status.go          ← Constantes tipadas + validación
│   └── user/
│       ├── update_request.go       ← DTO para PUT /users/{id} (punteros)
│       ├── update_response.go      ← DTO response para actualización
│       └── status_change_request.go ← DTO para PATCH .../status
├── controllers/
│   └── user_controller.go          ← Handlers Update + ChangeStatus
├── services/
│   └── user_service.go             ← Lógica de actualización y cambio de estado
├── daos/
│   └── user_dao.go                 ← Acceso a DB para gestión de usuarios
└── app/
    ├── app.go                      ← DI (nuevos servicios, DAOs, controladores)
    └── url_mappings.go             ← Nuevas rutas
```

## Risks / Trade-offs

- **Punteros en DTO de actualización**: Mayor complejidad en el controller (validar nil). Mitigación: patrones claros de mapeo en el service.
- **PUT sin cambios en status**: Puede confundir si el cliente envía status y se ignora. Mitigación: documentar en Swagger que status tiene endpoint dedicado.
- **Sin historial de cambios de estado**: Si se necesita auditoría después, habrá que migrar datos. Aceptado por ahora (non-goal).
- **Reutilización de FindByEmail**: Auth DAO ya tiene un `FindByEmail`. User DAO tendrá el suyo propio. Mitigación: mantener ambos para no acoplar capas.

## Open Questions

- ¿Transiciones de estado permitidas? Por ejemplo, de `active` a `blocked` sí pero de `blocked` a `active` solo por admin. Por ahora se permite cualquier transición entre estados válidos.
- ¿El endpoint PUT debe requerir todos los campos (reemplazo completo tipo REST) o solo los enviados (merge)? Se opta por merge (solo campos enviados) para mejor UX.
- ¿Login/autenticación se implementará en otro cambio? Sí, non-goal aquí.