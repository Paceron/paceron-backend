# Proposal

## Why

El resumen del dashboard no distingue el mejor tiempo registrado del mejor tiempo que además cumplió su objetivo, ni entrega los acumulados necesarios para comparar la distancia objetivo con la lograda. El frontend necesita esos valores para presentar los KPIs sin reconstruir las reglas de cumplimiento.

## Objetivo

Ampliar `totals` de `GET /api/v1/dashboard/metrics/summary` con métricas de tiempo y distancia relativas a objetivos, conservando intacta la semántica de los campos ya publicados.

## Alcance

- Agregar `totals.best_time_met_ms`: menor tiempo positivo de una serie cuyo `met_target` es `true`, evaluado con la tolerancia efectiva de la consulta.
- Agregar `totals.target_distance_m` y `totals.achieved_target_distance_m` para las series del scope cuyo tipo declara un objetivo de distancia.
- Incluir el objetivo de distancia de esas series aunque la serie no tenga una medición válida; una serie inválida aporta `0` al acumulado logrado.
- Devolver los campos de distancia como `null` cuando el scope no contiene objetivos de distancia. Si hay objetivos pero ninguna medición válida, devolver el objetivo acumulado y `0` como logrado.
- Mantener `totals.best_time_ms`, `totals.distance_m` y `totals.compliance_rate` con su comportamiento actual.

## No alcance

- Cambiar los KPIs de `timeseries`, leaderboard, comparaciones u otros endpoints.
- Cambiar las reglas de validez, `met_target`, `band_counts` o `compliance_rate`.
- Agregar un ratio calculado, modificar el esquema de base de datos o implementar cambios en el frontend.

## Métrica de éxito

- La respuesta del resumen conserva los resultados actuales de `best_time_ms` y `distance_m` y expone las tres nuevas claves de `totals`.
- Las pruebas verifican que el mejor tiempo usa `met_target` y la tolerancia solicitada, y que los objetivos de distancia de series inválidas permanecen en el denominador mientras su aporte logrado es cero.
- El contrato generado de Swagger refleja los nuevos campos y `go test ./...` pasa.

## What Changes

- Se amplía el contrato JSON del resumen con un mejor tiempo condicionado al cumplimiento y con los acumulados de distancia objetivo/lograda.
- Se calculan las métricas nuevas usando los objetivos congelados de las instancias y las reglas existentes de cumplimiento y validez, sin redefinir las métricas actuales.
- Se actualizan las pruebas del agregado y la documentación Swagger generada.

## Capabilities

### New Capabilities

- `dashboard-objective-metrics`: métricas de objetivos incluidas en los acumulados del resumen analítico del dashboard.

### Modified Capabilities

- Ninguna.

## Impact

- Código: `cmd/api/domains/dashboard/dto.go` y `cmd/api/domains/dashboard/aggregate.go`.
- Pruebas: `cmd/api/domains/dashboard/aggregate_test.go` y cobertura del contrato JSON.
- API: respuesta de `GET /api/v1/dashboard/metrics/summary`, con campos aditivos y nullable.
- Documentación: definiciones Swagger en `cmd/api/docs/`.
- No requiere migraciones ni dependencias nuevas.
