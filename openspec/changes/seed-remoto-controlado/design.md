## Contexto

`cmd/api/scripts/seed_dashboard/` es un programa Go que usa la config del backend (stage, DB, env vars). Hoy corre solo con `--stage=local` por el guard `AssertSeedingStage` en `cmd/api/config/seeding.go`. El stamping de histórico escribe fechas pasadas y pisa calendario, por eso el guard.

## Decisiones

### D1 — Flag explícito `--target`, no env var sola
El parámetro de escritura remota es un flag CLI, no solo una env var, para que quede visible en el comando y en el historial de shell. Env vars (`SEEDING_ALLOW_REMOTE`) solo habilitan la capacidad; el flag decide la ejecución.

### D2 — Dos env vars independientes
`SEEDING_ENABLED` habilita el stamping de histórico (como hoy). `SEEDING_ALLOW_REMOTE` habilita escribir fuera de local. Ambas deben estar en true para el modo remoto. Así un entorno con `SEEDING_ENABLED=true` (testing cloud, por error) no permite escritura remota sin el segundo opt-in.

### D3 — Producción nunca
Si `IsProductionStage()` y `--target=testing`, panic con mensaje claro. No hay override.

### D4 — Scope obligatorio en remoto
En modo remoto, `--team-id` y `--group-id` son requeridos (sin default). Reduce el riesgo de sembrar sobre un scope ajeno por olvido. `resolveScope` sigue validando que el grupo pertenezca al equipo.

### D5 — Ventana obligatoria en remoto
`--start` requerido en modo remoto. Evita depender de "esta semana" implícita.

### D6 — Confirmación interactiva + `--yes`
Antes de escribir, imprimir resumen (target, team, group, ventana, qué se crea) y pedir `yes`. `--yes` salta el prompt (para CI). Sin confirmación, aborta sin escribir.

### D7 — Idempotencia estricta en remoto
En remoto, si `existingCalendarDays()` ya tiene días con `source_plan_id` del seed en la ventana, abortar en vez de reutilizar. En local se mantiene reutilizar.

### D8 — Dry-run por default
Sin `--seed`, solo informa (igual que hoy). Con `--seed` ejecuta.

## Archivos a tocar

- `cmd/api/config/seeding.go` — agregar `SeedingAllowRemote` var + `loadSeedingAllowRemoteConfig()` + helper `AssertSeedingRemoteAllowed()`.
- `cmd/api/scripts/seed_dashboard/main.go` — parsear `--target`, validar guardas, confirmación.
- `cmd/api/scripts/seed_dashboard/seed.go` — `resolveScope`/`stamp` ajustes para modo remoto (scope obligatorio, idempotencia estricta).
- `cmd/api/scripts/seed_dashboard/RUNBOOK.md` — documentar modo remoto.

## Riesgos

- Escribir en testing cloud afecta datos compartidos entre devs. Mitigado por scope explícito + confirmación + dry-run default + idempotencia estricta.
- Si alguien setea `SEEDING_ALLOW_REMOTE=true` en prod por error, el guard de producción (D3) lo bloquea.