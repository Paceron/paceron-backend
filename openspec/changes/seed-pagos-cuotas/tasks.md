## Tasks

### A. Más días estampados en seed_dashboard
1. Cambiar `weekTargets` para cubrir más días/semana (ej. L/M/X/J/V) y más semanas.
2. Actualizar `seedSessionsCount` y `validateDays`.
3. Actualizar tests del seeder al nuevo conteo.

### B. Script seed_payments
4. Crear `cmd/api/scripts/seed_payments/` con main.go (flags, modo remoto, confirmación) reutilizando el patrón de seed_dashboard.
5. Implementar seed.go: crear cuotas por corredor con estados variados, idempotente.
6. Tests del script.
7. RUNBOOK.md.

### C. Verificación
8. `go test ./...` verde; dry-run y seed real en local.