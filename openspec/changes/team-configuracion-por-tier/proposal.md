## Why

Cuando el frontend arma/edita un equipo necesita saber cuántos integrantes máximos quiere permitir el creador y cuál es el `membership_fee` mínimo, y esto depende del **tier del entrenador** (el tier de su rol "entrenador"). Hoy ese dato no existe en la API: la validación "cuánto te deja tu plan" queda en el aire y el frontend no tiene de dónde sacar los topes por tier.

## What Changes

- **Nuevo endpoint `GET /api/v1/team-configuration`** (autenticado): recibe `user_id` (entrenador) y `team_id` por query y devuelve la configuración según el tier del entrenador:
  - `max_members`: cantidad máxima de integrantes.
  - `minimum_fee`: valor mínimo de `membership_fee`.
- **Hashmap en código** (`cmd/api/domains/teamconfiguration`): tier → configuración. Si el tier no está en el mapa (o el usuario no tiene tier resuelto), devuelve el **default**: `max_members = 10`, `minimum_fee = 20000`.
- El **tier se resuelve igual que `GET /users/:id/subscriptions/current`** para el rol "entrenador": suscripción vigente (`user_role_tier_subscriptions.active/first_payment_pending`) si existe; si no, el tier asignado en `user_roles.tier_id`.
- Se **valida que el entrenador sea dueño del equipo** (`teams.owner_id == user_id`); si el equipo no existe → `404`, si no es dueño → `403`.

## Capabilities

### New Capabilities
- `team-configuration`: consulta de configuración de equipo según el tier del entrenador.

### Modified Capabilities
<!-- No hay spec previa; la behavior queda cubierta por la capability nueva. -->

## Impact

- **API**: endpoint nuevo `GET /api/v1/team-configuration` + swagger regenerado.
- **Código**: `cmd/api/domains/teamconfiguration/` (hashmap + response), `cmd/api/services/team_configuration_service.go`, `cmd/api/controllers/team_configuration_controller.go`, wiring en `cmd/api/app/app.go`, ruta en `cmd/api/app/url_mappings.go`.
- **Tests**: unit del hashmap (`teamconfiguration`), `team_configuration_service_test.go` y `team_configuration_controller_test.go` (mocks existentes reutilizados).
- **Docs**: `README.md` (tabla de endpoints).
- **Sin cambios de schema** ni de flujos existentes (tiers, suscripciones, equipos).