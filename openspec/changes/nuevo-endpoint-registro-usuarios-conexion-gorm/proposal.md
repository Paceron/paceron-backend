## Why

El proyecto ya tiene GORM configurado y un endpoint básico `POST /user` para crear usuarios, pero carece de:
- Un flujo de registro propiamente dicho con hash de contraseña
- Migración automática de esquemas (auto-migrate)
- Una inicialización encapsulada del cliente DB siguiendo la nueva convención `clients/`

Este cambio sienta las bases para tener una conexión a DB correctamente encapsulada y un endpoint de registro listo para usar en producción.

## What Changes

- **DB client**: Agregar auto-migrate de modelos GORM en `infrastructure/postgresdb/postgres.go` y mejorar su configuración (pool, timeouts, health check).
- **Auto-migrate**: Ejecutar auto-migrate de modelos GORM al iniciar la conexión.
- **Modelo User**: Agregar campo `Email` al modelo DB y al DTO de registro.
- **Password hashing**: Incorporar bcrypt para hashear contraseñas en el registro.
- **Registro de usuarios**: Nuevo endpoint `POST /api/v1/auth/register` con validación de email único y password hasheado.
- **Arquitectura**: Mantener la inyección actual en `app.go` consumiendo `postgresdb.ConfigDB`.
- **Swagger**: Documentar el nuevo endpoint.

## Capabilities

### New Capabilities
- `user-registration`: Registro de usuarios con email, nombre y contraseña hasheada (bcrypt)

### Modified Capabilities
- *(Ninguna — es la primera capability del proyecto)*

## Impact

- **Modificar**: `cmd/api/infrastructure/postgresdb/postgres.go` — agregar auto-migrate y mejorar pool
- **Nuevo endpoint**: `POST /api/v1/auth/register`
- **Nuevo dominio**: `cmd/api/domains/auth/` — DTOs de request/response del registro
- **Servicio nuevo**: `cmd/api/services/auth_service.go` — lógica de registro
- **DAO nuevo**: `cmd/api/daos/auth_dao.go` o extender `user_dao.go`
- **Refactor**: `app.go` — agregar auto-migrate después de `postgresdb.ConfigDB`
- **Dependencia nueva**: `golang.org/x/crypto` para bcrypt
- **Config**: Las mismas env vars de DB existentes aplican (sin cambios)
