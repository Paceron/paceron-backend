# Prompt: Implementación de Teams, Groups y Invitation

## Objetivo

Implementar las features de **Teams**, **Groups** e **Invitation** en el backend de Paceron, siguiendo la arquitectura en capas existente (Controllers → Services → DAOs → Infrastructure) y todas las convenciones del proyecto.

## Prompt utilizado

```
Implementar Teams, Groups y Invitation en el backend de Paceron.

Arquitectura: Controllers → Services → DAOs → Infrastructure (Go 1.26 + Gin + GORM + JWT + Swagger).

Entidades:
- Team: nombre, descripción, nivel, max_members, requirements, owner_id (User), status (string: active/inactive/archived), address (country, province, city, street, number). Owner debe tener rol "entrenador". Un solo owner por team.
- Group: nombre, descripción, is_main, team_id (Team). Pertenecen a un team.
- TeamUser: asociación team ↔ user con rol (owner/coach/member), status, date_start, date_end.
- GroupUser: asociación group ↔ user, status, date_start, date_end.
- Invitation:邀请 existente por email a unirse a un team.

Reglas:
- Todos los deletes son soft deletes.
- Owner solo puede ser "entrenador".
- Invitation solo para usuarios existentes.
- Dirección del team se setea via endpoint dedicado PUT /teams/:id/address.

Endpoints (17):
Teams: POST/GET/GET/:id/PUT/DELETE + PUT /:id/address
TeamUsers: POST /:id/users + DELETE /:id/users/:user_id
Groups: POST/GET/GET/:id/PUT/DELETE
GroupUsers: POST /:id/users + DELETE /:id/users/:user_id
Invitation: POST /teams/:id/invite

Seguir convenciones del proyecto:
- DTOs en domains/{entity}/
- DAOs con interfaces
- Services con interfaces
- Controllers con interfaces
- Tests con mocks
- Swagger annotations en español
- GoDoc en español
- Constants para enums
```

## Resultado

### Archivos creados (FASE 1: Modelos y DAOs)
- `cmd/api/domains/dbs/team.go` — Modelo DB Team
- `cmd/api/domains/dbs/group.go` — Modelo DB Group
- `cmd/api/domains/dbs/team_user.go` — Modelo DB TeamUser
- `cmd/api/domains/dbs/group_user.go` — Modelo DB GroupUser
- `cmd/api/domains/constants/team_status.go` — Constantes de estado de team
- `cmd/api/domains/constants/team_status_test.go` — Tests de constantes
- `cmd/api/domains/constants/team_user_role.go` — Constantes de rol en team
- `cmd/api/domains/constants/team_user_role_test.go` — Tests de constantes
- `cmd/api/domains/team/` — DTOs request/response de Team
- `cmd/api/domains/group/` — DTOs request/response de Group
- `cmd/api/domains/teamuser/` — DTOs request/response de TeamUser
- `cmd/api/domains/groupuser/` — DTOs request/response de GroupUser
- `cmd/api/domains/invitation/` — DTOs request/response de Invitation
- `cmd/api/daos/team_dao.go` + `team_dao_test.go`
- `cmd/api/daos/group_dao.go` + `group_dao_test.go`
- `cmd/api/daos/team_user_dao.go` + `team_user_dao_test.go`
- `cmd/api/daos/group_user_dao.go` + `group_user_dao_test.go`

### Archivos creados (FASE 2: Services)
- `cmd/api/services/team_service.go` + `team_service_test.go`
- `cmd/api/services/group_service.go` + `group_service_test.go`
- `cmd/api/services/team_user_service.go` + `team_user_service_test.go`
- `cmd/api/services/group_user_service.go` + `group_user_service_test.go`
- `cmd/api/services/invitation_service.go` + `invitation_service_test.go`

### Archivos creados (FASE 3: Controllers)
- `cmd/api/controllers/team_controller.go` + `team_controller_test.go`
- `cmd/api/controllers/group_controller.go` + `group_controller_test.go`
- `cmd/api/controllers/team_user_controller.go` + `team_user_controller_test.go`
- `cmd/api/controllers/group_user_controller.go` + `group_user_controller_test.go`
- `cmd/api/controllers/invitation_controller.go` + `invitation_controller_test.go`

### Archivos modificados (FASE 4: Wiring)
- `cmd/api/app/app.go` — DI para nuevos controllers/services/DAOs
- `cmd/api/app/url_mappings.go` — 17 rutas registradas
- `cmd/api/infrastructure/mailer/render.go` — Template de invitación
- `cmd/api/infrastructure/mailer/mailer.go` — SendInvitationEmail
- `cmd/api/infrastructure/mailer/templates/invitation.html` — Template HTML
- `cmd/api/docs/` — Swagger regenerado
- `README.md` — Endpoints y diagrama de arquitectura actualizados

### Coverage final
- `controllers`: 99.4%
- `services`: 92.3%
- `constants`: 100%

### Patrón de decisiones de diseño
1. **Status de Team como string** (no tabla separada): consistente con `User.Status`, simple, sin JOIN necesario.
2. **Address embedded en Team**: mismo patrón que User, sin tabla separada.
3. **Constants para enums**: `team_status.go`, `team_user_role.go` — valores acordados y testeados.
4. **Template de email**: sigue el patrón de `welcome.html` y `reset.html` (embed, render function, data struct).
5. **Mocks reutilizados**: `mockRoleDao`, `mockTeamDao`, `mockUserDaoForUserRole` — no se crearon mocks duplicados.
