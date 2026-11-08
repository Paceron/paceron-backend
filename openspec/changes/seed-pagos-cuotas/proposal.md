## Why

El seeder `seed_dashboard` no cubre el dominio de pagos/cuotas. Para tener datos de prueba realistas en testing (y para el dashboard/feature de cobranzas) hace falta sembrar cuotas de los corredores del roster, con distintos estados (pagada, pendiente, vencida, cancelada) y montos, idempotente.

Además, el dataset actual estampa solo martes y jueves × 2 semanas (4 sesiones). Se quiere cubrir más días para tener más volumen de series y feedback.

## What Changes

- **Nuevo script `cmd/api/scripts/seed_payments/`** (programa Go separado, mismo patrón que `seed_dashboard`): siembra cuotas de los corredores del grupo (team_id/group_id seleccionables), con estados variados y montos, idempotente por (user, team, installment_number).
- **Ampliación del seed_dashboard**: estampar más días (configurable, default más días por semana / más semanas), manteniendo la validación de `validateDays` ajustada al nuevo conteo.
- **Ambos scripts corren con `--target=testing`** (modo remoto ya implementado) y escriben en la misma DB.

## Non-Goals

- No tocar el dominio de pagos (servicios/controllers). Solo inserta filas vía DAO, como hace el seeder de dashboard con el calendario.
- No integrar Mercado Pago real. Solo datos mock.

## Capabilities

### Added
- `seed-payments` (script + modo remoto).

### Modified
- `seed-dashboard` (más días estampados).