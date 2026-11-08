## Why

El entrenador necesita ver, en la pestaña Corredores de su equipo, el estado de cuota de membresía de cada corredor. Hoy `GET /api/v1/users/{id}/teams/{team_id}/subscription` es self-only: el backend ignora el `:id` del path y usa el usuario del JWT. No existe una variante para que el owner/entrenador consulte a otros miembros, y pedirla uno por corredor generaría N+1.

## What Changes

- Nuevo endpoint batch `GET /api/v1/teams/{id}/subscriptions`, restringido al owner del equipo o a un `team_user` con rol `entrenador`.
- Devuelve, en una sola respuesta, el estado de cuota de todos los corredores activos del equipo.
- Estados de presentación derivados: `active` (al día o equipo gratis), `first_payment_pending` (primer pago pendiente) y `overdue` (deuda vencida).
- Incluye `has_debt`, `subscription_status` crudo, `init_amount` y `paid_installments` por corredor.
- No modifica el endpoint self-only existente ni el shape del roster.
- Sin migración de base de datos.

## Capabilities

- `team-subscriptions`: nuevo endpoint batch de estado de cuota por equipo.

## Impact

- Backend: DAO de cuotas (consulta batch), service/controller y una ruta nueva.
- Frontend: podrá conectar `subscriptionStatus` del roster en una llamada por equipo; no hay breaking changes.
- Sin cambios en pagos, Mercado Pago ni contratos existentes.

## Non-Goals

- No se agregan nombres, avatares ni datos del roster (el frontend ya los tiene).
- No se cambia la regla de deuda ni el estado de cuenta individual.
- No se incluye al owner ni a otros entrenadores en la lista de corredores.
