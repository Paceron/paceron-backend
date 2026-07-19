## ADDED Requirements

### Requirement: El sistema SHALL retornar los permisos de un usuario

El sistema SHALL aceptar solicitudes GET a `/api/v1/auth/permissions?user_id=<id>` y retornar la estructura con los roles, tiers y permisos asignados al usuario.

#### Scenario: Consultar permisos con datos completos
- **WHEN** se envía una solicitud GET a `/api/v1/auth/permissions?user_id=123` y el usuario tiene roles asignados con tiers y permisos
- **THEN** el sistema responde con HTTP 200 y la estructura:
```json
{
  "user_id": 123,
  "roles": [
    {
      "id": 54667,
      "name": "corredor",
      "tier": "pro",
      "permissions": ["permission_1", "permission_2"]
    }
  ]
}
```

#### Scenario: Consultar permisos sin user_id
- **WHEN** se envía una solicitud GET a `/api/v1/auth/permissions` sin parámetro `user_id`
- **THEN** el sistema responde con HTTP 400 y mensaje "El parámetro user_id es requerido"

#### Scenario: Consultar permisos de usuario inexistente
- **WHEN** se envía una solicitud GET a `/api/v1/auth/permissions?user_id=99999` y el usuario no existe
- **THEN** el sistema responde con HTTP 404 y mensaje "Usuario no encontrado"

#### Scenario: Consultar permisos de usuario sin roles asignados
- **WHEN** se envía una solicitud GET a `/api/v1/auth/permissions?user_id=123` y el usuario no tiene roles asignados
- **THEN** el sistema responde con HTTP 200 con `user_id` y `roles` como array vacío

#### Scenario: Consultar permisos con datos faltantes
- **WHEN** se envía una solicitud GET a `/api/v1/auth/permissions?user_id=123` y un rol asignado no tiene tier o un tier no tiene permisos
- **THEN** el sistema registra logs de error con los datos faltantes y responde con HTTP 400 y mensaje descriptivo indicando qué datos faltan configurar

### Requirement: El sistema SHALL validar la integridad de la cadena de permisos

El sistema SHALL verificar que cada rol asignado al usuario tenga un tier válido y que cada tier tenga al menos un permiso asociado.

#### Scenario: Rol sin tier asignado
- **WHEN** un rol asignado al usuario no tiene un `tier_id` válido en `user_roles`
- **THEN** el sistema registra un log de error con `user_id`, `role_id` y mensaje "Tier no configurado para el rol"

#### Scenario: Tier sin permisos asociados
- **WHEN** un tier del usuario no tiene permisos en `tier_permissions`
- **THEN** el sistema registra un log de error con `user_id`, `tier_id` y mensaje "El tier no tiene permisos asociados"
