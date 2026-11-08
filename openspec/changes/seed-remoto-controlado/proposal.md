## Why

El seeder `cmd/api/scripts/seed_dashboard/` está pensado exclusivamente para la base local: `AssertSeedingStage` hace panic si `SEEDING_ENABLED=true` fuera de `--stage=local`, y el stamping de histórico requiere ese flag. No hay camino soportado para sembrar la base de testing cloud de Render, aunque sea deliberado y con scope acotado. Para poblar testing con datos de demo/prueba hay que hacer dump/restore manual, que es frágil y no respeta la idempotencia del seeder.

Se quiere habilitar, de forma **explícita y opt-in**, la escritura remota a la base de testing, tomando como base el seeder existente (equipo/grupo/fechas ya seleccionables), con guardas que hagan el riesgo aceptable: flag explícito al ejecutar, scope obligatorio, ventana de fechas obligatoria, confirmación interactiva, dry-run por default e idempotencia estricta.

## What Changes

- **Nuevo flag CLI `--target=local|testing`** (default `local`). En `local` el comportamiento no cambia.
- **Modo remoto (`--target=testing`)** habilitado solo si además `SEEDING_ENABLED=true` y un nuevo env var `SEEDING_ALLOW_REMOTE=true`. Si el stage resuelto es `production`, panic (igual que hoy para local).
- **Scope obligatorio en remoto**: `--team-id` y `--group-id` dejan de tener default; sin ellos, aborta. `resolveScope` se mantiene y valida que el grupo pertenezca al equipo.
- **Ventana de fechas obligatoria en remoto**: `--start=YYYY-MM-DD` sin default.
- **Confirmación interactiva**: resumen + prompt `yes` (o `--yes` para CI). Sin confirmación, aborta sin escribir.
- **Dry-run por default**: sin `--seed`, solo informa (igual que hoy).
- **Idempotencia estricta en remoto**: si ya existen días del seed en la ventana (`source_plan_id` del seed), aborta en vez de reutilizar. En local se mantiene el comportamiento actual (reutilizar).
- **Documentación**: actualizar `RUNBOOK.md` del seeder con el modo remoto y sus guardas.

## Non-Goals

- Permitir escribir en producción. Nunca.
- Cambiar el modelo de datos o el stamping. Solo se habilita el target.
- El feature de pagos de cuotas (pedido aparte) no se incluye acá; va en otra spec.