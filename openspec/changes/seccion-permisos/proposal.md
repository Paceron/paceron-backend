## Why

La aplicación necesita un sistema de permisos que permita controlar el acceso de los usuarios a diferentes funcionalidades. Actualmente no existe una estructura de roles, tiers ni permisos que regule qué puede hacer cada usuario. Este sistema es fundamental para escalar la aplicación y agregar funcionalidades de forma segura.

## What Changes

- Se crean las entidades **Permission**, **Tier** y **Role** como entidades independientes con ABM (POST/PUT/DELETE lógico).
- Se crea la entidad **TierPermissions** como tabla intermedia para asociar permisos a tiers.
- Se crea la entidad **UserRole** para asignar roles a usuarios, con tier por defecto "base".
- Se agrega endpoint `GET /api/v1/auth/permissions` que retorna los permisos asignados a un usuario a través de sus roles y tiers.
- Se registra la migración de las nuevas tablas en el AutoMigrate de GORM.
- Se agregan controllers, services y DAOs siguiendo la arquitectura en capas existente.

## Capabilities

### New Capabilities

- `permission-management`: ABM de permisos como entidad independiente (crear, actualizar, eliminar lógico).
- `tier-management`: ABM de tiers como entidad dependiente de un rol (crear, actualizar, eliminar lógico). Cada tier pertenece a un rol y tiene un nombre único dentro de ese rol.
- `role-management`: ABM de roles como entidad independiente (crear, actualizar, eliminar lógico).
- `tier-permission-association`: Asignación y desasignación de permisos a tiers.
- `user-role-assignment`: Asignación de roles a usuarios con tier por defecto "base".
- `user-permissions-query`: Consulta de permisos de un usuario agrupados por rol y tier.

### Modified Capabilities

- `user-login`: Se modifica la respuesta del endpoint `GET /api/v1/auth/permissions` para incluir la consulta de permisos del usuario (nuevo endpoint, no modifica login existente).

## Impact

- **Archivos nuevos**: Modelos GORM en `domains/dbs/`, DTOs en `domains/permission/`, `domains/tier/`, `domains/role/`, `domains/userrole/`, `domains/tierpermission/`. DAOs, services y controllers nuevos. Migración en `infrastructure/postgresdb/postgres.go`. Rutas en `app/url_mappings.go`. Inyección de dependencias en `app/app.go`.
- **Archivos modificados**: `infrastructure/postgresdb/postgres.go` (AutoMigrate), `app/app.go` (DI), `app/url_mappings.go` (rutas).
- **APIs**: 13 nuevos endpoints REST.
- **Dependencias**: No se agregan dependencias externas nuevas.
- **Base de datos**: 5 nuevas tablas (`permissions`, `tiers`, `roles`, `tier_permissions`, `user_roles`).
