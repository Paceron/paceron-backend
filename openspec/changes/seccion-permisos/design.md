## Context

El proyecto Paceron backend utiliza una arquitectura en capas (Controllers → Services → DAOs → DB) con Go, Gin, GORM y PostgreSQL. Actualmente solo existe la entidad User con sus operaciones CRUD. Se necesita agregar un sistema de permisos completo que incluya las entidades Permission, Tier, Role y sus tablas intermedias.

El diagrama de dominio (DOM2.drawio.html) define las relaciones entre estas entidades. La decisión clave es que cada Tier pertenece a un Role (N:1), y los permisos llegan al usuario a través de la cadena User → UserRole → Role → Tier → TierPermissions → Permission.

## Goals / Non-Goals

**Goals:**
- Crear las 5 tablas nuevas siguiendo el patrón de migración existente (AutoMigrate en postgres.go)
- Implementar ABM completo (POST/PUT/DELETE lógico) para Permission, Tier y Role
- Implementar asociación de permisos a tiers (POST/DELETE)
- Implementar asignación de roles a usuarios con tier por defecto "base"
- Implementar endpoint GET /api/v1/auth/permissions con la estructura solicitada
- Tests con >90% de coverage
- Documentación Swagger actualizada

**Non-Goals:**
- No se implementa autenticación/autorización por permisos (solo la estructura de datos)
- No se modifica la entidad User existente
- No se agregan dependencias externas nuevas
- No se implementa paginación en los endpoints de consulta (se puede agregar después)

## Decisions

### 1. Modelo de datos: Tier con role_id obligatorio

**Decisión:** Cada registro en `tiers` tiene un `role_id` (FK a `roles`) y un `role_name` (redundante, para simplificar consultas).

**Alternativas consideradas:**
- Tabla `role_tiers` como N:N: Rechazada porque el diagrama y las reglas de negocio indican que un tier es específico de un rol. "premium" de "corredor" ≠ "premium" de "entrenador".

**Justificación:** Un tier no tiene sentido sin un rol. La redundancia de `role_name` evita JOINs frecuentes en el endpoint de permisos.

### 2. Soft delete con campo `deleted_at`

**Decisión:** Todas las tablas nuevas incluyen `deleted_at *time.Time` para soft delete. Los queries filtran con `WHERE deleted_at IS NULL`.

**Alternativas consideradas:**
- Campo `is_deleted bool`: Rechazado porque no preserves cuándo se eliminó.
- GORM `DeleteStrategy`: No se usa porque el proyecto no tiene esta configuración.

**Justificación:** Consistente con el patrón que se usará para la entidad User (cuando se agregue soft delete). Permite auditoría y recuperación.

### 3. Arquitectura: seguir patrón de capas existente

**Decisión:** Por cada dominio nuevo (permission, tier, role, userrole, tierpermission), crear:
- `domains/dbs/<entity>.go` — Modelo GORM
- `domains/<entity>/` — DTOs de request/response
- `daos/<entity>_dao.go` — Interfaz + implementación
- `services/<entity>_service.go` — Interfaz + implementación
- `controllers/<entity>_controller.go` — Interfaz + implementación

**Alternativas consideradas:**
- Un solo controller/service para todos los permisos: Rechazado porque violaría el principio de responsabilidad única y dificultaría los tests.

**Justificación:** Consistente con la arquitectura existente. Cada capa tiene una responsabilidad clara.

### 4. Endpoint GET /api/v1/auth/permissions: delegación en auth

**Decisión:** El endpoint de consulta de permisos se ubica bajo `/api/v1/auth/` y utiliza un delegate que orquesta los DAOs de user, user_role, role, tier y permission.

**Alternativas consideradas:**
- Crear un controller independiente `/api/v1/permissions/user`: Rechazado porque la consulta está ligada al contexto de autenticación.

**Justificación:** El prefijo `/api/v1/auth/` es coherente con el dominio de consulta de permisos del usuario autenticado.

### 5. Validación de tier por defecto "base"

**Decisión:** Al asignar un rol a un usuario sin `tier_id`, se busca el tier con nombre "base" para ese rol. Si no existe, se retorna error.

**Alternativas consideradas:**
- Crear automáticamente el tier "base": Rechazado porque puede causar datos inesperados.

**Justificación:** Fuerza a que cada rol tenga un tier "base" configurado explícitamente antes de poder asignar roles sin tier explícito.

### 6. Nombre único de tier por rol

**Decisión:** Se crea un índice único compuesto en la tabla `tiers` sobre `(name, role_id)` donde `deleted_at IS NULL`.

**Alternativas consideradas:**
- Validar en código únicamente: Rechazado porque puede causar race conditions.

**Justificación:** La restricción a nivel de base de datos garantiza integridad incluso con concurrencia.

## Risks / Trade-offs

- **[Risk]** AutoMigrate puede causar problemas en producción → **Mitigación:** Solo se usa en desarrollo. Para producción se generarán migraciones manuales con `go migrate`.
- **[Risk]** La redundancia de `role_name` en `tiers` puede causar inconsistencias → **Mitigación:** Se actualiza `role_name` siempre que se actualice el nombre del rol (o se decide no permitir renaming de roles).
- **[Risk]** Queries complejos para el endpoint de permisos pueden ser lentos → **Mitigación:** Se usan índices en las FKs y se puede cachear el resultado en el futuro.
- **[Trade-off]** No se implementa autorización por permisos (solo la estructura) → Se puede agregar después como un middleware que consulte el endpoint de permisos.
