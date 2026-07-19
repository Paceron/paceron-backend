## 1. Modelos de Dominio (domains/dbs/)

- [x] 1.1 Crear `domains/dbs/permission.go` con modelo GORM (id, name, description, deleted_at, created_at, updated_at) y comentarios explicativos
- [x] 1.2 Crear `domains/dbs/tier.go` con modelo GORM (id, name, description, role_id, role_name, deleted_at, created_at, updated_at) y comentarios explicativos
- [x] 1.3 Crear `domains/dbs/role.go` con modelo GORM (id, name, description, deleted_at, created_at, updated_at) y comentarios explicativos
- [x] 1.4 Crear `domains/dbs/tier_permission.go` con modelo GORM (id, tier_id, permission_id, asignation_date, deleted_at, created_at, updated_at) y comentarios explicativos
- [x] 1.5 Crear `domains/dbs/user_role.go` con modelo GORM (id, user_id, role_id, tier_id, assignment_date, date_since, date_to, status, deleted_at, created_at, updated_at) y comentarios explicativos

## 2. DTOs de Request/Response

- [x] 2.1 Crear `domains/permission/permission_request.go` con CreatePermissionRequest y UpdatePermissionRequest
- [x] 2.2 Crear `domains/permission/permission_response.go` con PermissionResponse y DeletePermissionResponse
- [x] 2.3 Crear `domains/tier/tier_request.go` con CreateTierRequest y UpdateTierRequest
- [x] 2.4 Crear `domains/tier/tier_response.go` con TierResponse y DeleteTierResponse
- [x] 2.5 Crear `domains/role/role_request.go` con CreateRoleRequest y UpdateRoleRequest
- [x] 2.6 Crear `domains/role/role_response.go` con RoleResponse y DeleteRoleResponse
- [x] 2.7 Crear `domains/tierpermission/tier_permission_request.go` con AssignPermissionRequest
- [x] 2.8 Crear `domains/tierpermission/tier_permission_response.go` con TierPermissionResponse y DeleteTierPermissionResponse
- [x] 2.9 Crear `domains/userrole/user_role_request.go` con AssignRoleRequest
- [x] 2.10 Crear `domains/userrole/user_role_response.go` con UserRoleResponse

## 3. Migración de Base de Datos

- [x] 3.1 Actualizar `infrastructure/postgresdb/postgres.go` para agregar AutoMigrate de Permission, Tier, Role, TierPermission, UserRole

## 4. DAOs

- [x] 4.1 Crear `daos/permission_dao.go` con interfaz y implementación (Create, FindByID, FindByName, Update, SoftDelete)
- [x] 4.2 Crear `daos/tier_dao.go` con interfaz y implementación (Create, FindByID, FindByNameAndRole, Update, SoftDelete)
- [x] 4.3 Crear `daos/role_dao.go` con interfaz y implementación (Create, FindByID, FindByName, Update, SoftDelete)
- [x] 4.4 Crear `daos/tier_permission_dao.go` con interfaz y implementación (Create, FindByTierAndPermission, SoftDelete)
- [x] 4.5 Crear `daos/user_role_dao.go` con interfaz y implementación (Create, FindByUserAndRole, FindByUserID, SoftDelete)

## 5. Services

- [x] 5.1 Crear `services/permission_service.go` con interfaz y implementación (Create, Update, Delete) + validaciones
- [x] 5.2 Crear `services/tier_service.go` con interfaz y implementación (Create, Update, Delete) + validaciones + obtención de role_name
- [x] 5.3 Crear `services/role_service.go` con interfaz y implementación (Create, Update, Delete) + validaciones
- [x] 5.4 Crear `services/tier_permission_service.go` con interfaz y implementación (Assign, Unassign) + validaciones
- [x] 5.5 Crear `services/user_role_service.go` con interfaz y implementación (AssignRole) + lógica de tier por defecto "base"
- [x] 5.6 Crear `services/permissions_query_service.go` con interfaz y implementación para GET /api/v1/auth/permissions + validación de integridad

## 6. Controllers

- [x] 6.1 Crear `controllers/permission_controller.go` con interfaz y implementación (Create, Update, Delete)
- [x] 6.2 Crear `controllers/tier_controller.go` con interfaz y implementación (Create, Update, Delete)
- [x] 6.3 Crear `controllers/role_controller.go` con interfaz y implementación (Create, Update, Delete)
- [x] 6.4 Crear `controllers/tier_permission_controller.go` con interfaz y implementación (Assign, Unassign)
- [x] 6.5 Crear `controllers/user_role_controller.go` con interfaz y implementación (AssignRole)
- [x] 6.6 Crear `controllers/permissions_query_controller.go` con interfaz y implementación (GetUserPermissions)

## 7. Inyección de Dependencias y Rutas

- [x] 7.1 Actualizar `app/app.go` para inyectar los nuevos DAOs, services y controllers
- [x] 7.2 Actualizar `app/url_mappings.go` para registrar los 13 nuevos endpoints

## 8. Tests

- [x] 8.1 Crear tests para `daos/permission_dao.go`
- [x] 8.2 Crear tests para `daos/tier_dao.go`
- [x] 8.3 Crear tests para `daos/role_dao.go`
- [x] 8.4 Crear tests para `daos/tier_permission_dao.go`
- [x] 8.5 Crear tests para `daos/user_role_dao.go`
- [x] 8.6 Crear tests para `services/permission_service.go`
- [x] 8.7 Crear tests para `services/tier_service.go`
- [x] 8.8 Crear tests para `services/role_service.go`
- [x] 8.9 Crear tests para `services/tier_permission_service.go`
- [x] 8.10 Crear tests para `services/user_role_service.go`
- [x] 8.11 Crear tests para `services/permissions_query_service.go`
- [x] 8.12 Crear tests para `controllers/permission_controller.go`
- [x] 8.13 Crear tests para `controllers/tier_controller.go`
- [x] 8.14 Crear tests para `controllers/role_controller.go`
- [x] 8.15 Crear tests para `controllers/tier_permission_controller.go`
- [x] 8.16 Crear tests para `controllers/user_role_controller.go`
- [x] 8.17 Crear tests para `controllers/permissions_query_controller.go`
- [x] 8.18 Ejecutar `go test ./...` y verificar coverage >90%

## 9. Documentación y Swagger

- [x] 9.1 Ejecutar `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs` para regenerar Swagger
- [x] 9.2 Verificar que todos los endpoints aparecen correctamente en Swagger UI
- [x] 9.3 Actualizar README.md con los nuevos endpoints en la tabla

## 10. Colección Bruno

- [x] 10.1 Crear colección Bruno con todos los endpoints para testing de carga de datos
