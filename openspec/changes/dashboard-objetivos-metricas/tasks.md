# Tasks

## 1. Mejor tiempo condicionado al cumplimiento

- [x] 1.1 Añadir pruebas de `BuildSummary` para una serie más rápida que no cumple y otra más lenta que sí, para tolerancias distintas y para ausencia de un tiempo cumplidor; ejecutar `go test ./cmd/api/domains/dashboard -run TestBuildSummary_BestTimeMet` y confirmar primero el fallo contra el comportamiento actual y después su aprobación con la implementación.
- [x] 1.2 Agregar `best_time_met_ms` nullable y calcularlo con `metTarget` y la tolerancia recibida por `BuildSummary`, manteniendo intacto `best_time_ms`; verificar con `go test ./cmd/api/domains/dashboard -run TestBuildSummary_BestTimeMet`.

## 2. Acumulados de distancia objetivo

- [x] 2.1 Añadir pruebas para la suma de objetivos de filas válidas e inválidas, objetivo existente con cero logrado y ausencia de objetivos de distancia; ejecutar `go test ./cmd/api/domains/dashboard -run 'TestBuildSummary_(ObjectiveDistance|NoObjectiveDistance)'` y confirmar que los casos nuevos fallan antes del cálculo y pasan después.
- [x] 2.2 Agregar `target_distance_m` y `achieved_target_distance_m` nullable al resumen, contando objetivos positivos de todos los rows de distancia y distancia lograda solo de filas válidas, sin alterar los acumuladores usados por `deltaPct`; verificar con `go test ./cmd/api/domains/dashboard -run 'TestBuildSummary_(ObjectiveDistance|NoObjectiveDistance)'` y las pruebas existentes de `deltaPct`.

## 3. Contrato y verificación final

- [x] 3.1 Añadir una prueba de serialización para confirmar que las tres claves nuevas se emiten como `null` cuando no hay valor y regenerar Swagger con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`; verificar la prueba de JSON y que los esquemas generados incluyen `best_time_met_ms`, `target_distance_m` y `achieved_target_distance_m`.
- [x] 3.2 Ejecutar `go test ./...` y `git diff --check`; confirmar que toda la suite pasa y que el diff no contiene errores de whitespace.
