## Why

`teams.membership_fee` existe en la base y gobierna todo el flujo de membresía de equipo (gate de primer pago, cuotas, split), pero no está expuesto en la API de equipos: `POST /teams`, `PUT /teams/:id` y `GET /teams/:id` lo ignoran por completo. Hoy el único modo de setear el valor es un `UPDATE` por SQL contra la base (documentado como workaround en `docs/CU/02-pago-participacion-equipo.md` y `docs/CAMBIO_SUSCRIPCION_TEAMS_SPLIT_TESTING.md`), y el entrenador no puede ni leer cuánto cobra ni configurar su mensualidad desde la app.

## What Changes

- **`POST /api/v1/teams`**: el request `CreateTeamRequest` acepta `membership_fee` (opcional, `0` = equipo gratis). Se valida que sea `>= 0`; valores negativos → `400`.
- **`PUT /api/v1/teams/:id`**: el request `UpdateTeamRequest` acepta `membership_fee` (opcional, actualización parcial como el resto de los campos). Misma validación `>= 0`.
- **`GET /api/v1/teams/:id`** (y `GET /api/v1/teams`, `GetAll`): `TeamResponse` incluye `membership_fee`.
- El cambio de `membership_fee` **no altera retrospectivamente** membresías existentes: `team_users.init_amount` congeló el valor al momento de la unión (el gate usa `membership_fee` actual solo al dar de alta miembros nuevos).
- No cambian los endpoints de alta de miembros, invitaciones, ni suscripción de equipo; solo queda configurable el dato que esos flujos ya consumen.

## Capabilities

### New Capabilities
- `teams-management`: gestión CRUD de equipos exponiendo `membership_fee` (crear, actualizar y leer).

### Modified Capabilities
<!-- No hay spec previa de equipos; la behavior queda cubierta por la capability nueva. -->

## Impact

- **API**: contratos de `POST /teams`, `PUT /teams/:id` y `GET /teams/:id`; swagger regenerado.
- **Código**: `cmd/api/domains/team/team_request.go`, `team_update_request.go`, `team_response.go`; `cmd/api/services/team_service.go` (`Create`, `Update`, `toResponse`); godoc de `cmd/api/controllers/team_controller.go`.
- **Tests**: `team_service_test.go` (create/update con fee, validación negativo, respuesta) y `team_controller_test.go` si corresponde.
- **Docs**: `README.md` (tabla de endpoints), `docs/CU/02-pago-participacion-equipo.md` y `docs/CAMBIO_SUSCRIPCION_TEAMS_SPLIT_TESTING.md` dejan de instruir el workaround SQL (actualizar a "por API").
- **Sin cambios de schema**: la columna `teams.membership_fee` ya existe.