## Contexto

`Installment` (dbs.Installment) es la entidad de cuotas: arco exclusivo subscription (tier o team), con `team_id`, `user_id`, `installment_number`, `amount`, `status` (pending/paid/canceled), `due_date`, `paid_at`, `internal_payment_id`, `external_payment_id`. DAO: `daos.NewInstallmentDao`. Constantes: `constants.InstallmentStatus*`.

## Decisiones

### D1 — Script separado, no etapa del dashboard
El usuario quiere un script independiente que también impacte la misma DB. Permite correr pagos sin re-sembrar el dashboard. Comparte el modo remoto (`--target`) y el scope (team/group).

### D2 — Cuotas por corredor del roster
Para cada corredor activo del grupo, crear N cuotas (default 3) con montos del `membership_fee` del team (o un monto mock si no hay). Estados distribuidos: ~60% paid, ~25% pending, ~10% canceled, ~5% overdue (pending con due_date pasada).

### D3 — Idempotencia
Por (team_id, user_id, installment_number): si ya existe, reutilizar (no duplicar). En modo remoto, estricta: abortar si ya existen cuotas del seed en el scope.

### D4 — Más días estampados en el dashboard
Cambiar `weekTargets` para estampar L/M/X/J/V o ampliar semanas. Default: 3 días/semana × 3 semanas = 9 sesiones. Ajustar `seedSessionsCount` y `validateDays` (mínimo de bloques principales). Los tests del seeder deben actualizarse al nuevo conteo.

### D5 — Confirmación y dry-run
Igual que el seed_dashboard: dry-run por default, `--seed` para escribir, `--yes` para CI, confirmación interactiva en remoto.

## Archivos a crear/modificar

- Nuevo: `cmd/api/scripts/seed_payments/main.go`, `seed.go`, `RUNBOOK.md`.
- Modificar: `cmd/api/scripts/seed_dashboard/seed.go` (weekTargets, seedSessionsCount, validateDays) y tests.
- `cmd/api/scripts/seed_dashboard/main.go` ya soporta `--target`. Reutilizar el mismo patrón.

## Riesgos

- Ampliar días rompe tests que asumen 4 sesiones. Actualizar tests, no bajar umbral de coverage.
- El seed de pagos inserta cuotas sin pasar por el service de suscripciones; es mock data, documentar.