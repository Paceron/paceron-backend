# Tasks

> Coordinación: el frontend `dashboard-analitico-rendimiento` depende de este endpoint. Completar y desplegar la API antes de activar el consumo real; no reimplementar estas agregaciones en frontend.

## 1. Contrato HTTP y DTOs

- [x] 1.1 Definir DTOs `SessionComparisonRequest`, `SessionExerciseRef`, `SessionComparisonResponse`, resumen/lado, punto de serie, extremos y delta en `cmd/api/domains/dashboard/dto.go`; aplicar `binding:"required"` a IDs/objetos y mantener valores métricos nullable. Verificado con `gofmt` y `go test ./cmd/api/domains/dashboard`.
- [x] 1.2 Implementar `POST /api/v1/dashboard/comparisons/sessions` en controller con autenticación, `bindDashboardBody`, response/error uniforme y anotaciones Swagger de parámetros, respuestas `200/400/401/403/404/500`. Tests controller: request válida, body incompleto/campo desconocido/ID cero, auth y mapeo de errores.
- [x] 1.3 Registrar ruta en `cmd/api/app/url_mappings.go` y actualizar comentario de inventario de endpoints/permisos sin alterar el owner-only de las comparaciones existentes.

## 2. Scope y consulta de las referencias

- [x] 2.1 Añadir `SessionComparison` a `DashboardServiceInterface` y validar `team_id`, fechas/rango máximo, estructura e IDs antes de consultar las series. Cubierto por tests de rango y referencias inválidas/no encontradas.
- [x] 2.2 Validar cada lado dentro del mismo team: grupo del scope, atleta autorizado para ese grupo, IDs de sesión/ejercicio y membresía histórica en la fecha del feedback. Para miembros, exigir `athlete_user_id == callerID` en A y B; entrenadores pueden comparar uno o dos atletas del scope. Service tests de roles/scope y DAO tests existentes de ventana histórica.
- [x] 2.3 Resolver ejercicio de catálogo por `source_exercise_id` y conservar el fallback legacy de nombre normalizado del mismo owner. Rechazar referencias que no tengan feedback principal elegible; se reusa `resolveLegacyInstances` y el filtro de series del dashboard.
- [x] 2.4 Añadir filtro DAO de sesión exacta en `DashboardScopeFilter`; el query aplica grupo/atleta/sesión/ejercicio/rango y conserva los predicados existentes de membresía, rol y status en SQL. Tests de dashboard DAO pasan.
- [x] 2.5 Incluir `rpe` nullable de `workout_feedback` en el row DAO/domain necesario para el agregado, sin modificar migraciones ni constraints. Verificado en test DAO de selección exacta de sesión.
- [x] 2.6 Resolver nombre de grupo, atleta y ejercicio para el contexto de A/B, y asegurar que el error del segundo lado no permita serializar ni filtrar el primero. Verificado con tests de trainer/runner, nombres y fallo del lado B.

## 3. Agregación y respuesta

- [x] 3.1 Implementar función pura de agregado por lado que reusa `Series.Valid`, `Achieved`, `AchievedTimeMs` y velocidad del dominio: series registradas/válidas, distancia válida y velocidad media ponderada por distancia. Verificado en `go test ./cmd/api/domains/dashboard`.
- [x] 3.2 Calcular `mean_rpe` solo sobre RPE no nulo 1–10 e incluir `rpe_count`; ausencia de RPE no altera los KPIs ni la validez de rendimiento. Verificado con RPE parcial/ausente.
- [x] 3.3 Calcular `best`/`worst` por tipo: ritmo continuo por menor/mayor tiempo logrado con distancia; velocidad-distancia por mayor/menor velocidad derivada; velocidad-tiempo por mayor/menor distancia lograda. Ignorar filas no válidas para esos extremos y desempatar determinísticamente por fecha/set/feedback ID.
- [x] 3.4 Formar puntos de serie con IDs, número de serie, tiempos, distancia, velocidad derivada y RPE; ordenar por `set_number` y feedback ID. Serializar métricas no calculables como `null`, no `0`.
- [x] 3.5 Calcular deltas explícitos B−A: `speed_delta_kph`, `speed_delta_pct` con null cuando no haya denominador positivo, y `distance_delta_m`; declarar dirección y unidades.

## 4. Pruebas

- [x] 4.1 Tests unitarios del dominio para los tres kinds, validez por kind, extremos, empate determinista, ponderación de velocidad, suma de distancia, RPE parcial/ausente y deltas positivos/negativos/null. `go test ./cmd/api/domains/dashboard` pasa.
- [ ] 4.2 Tests de DAO con PostgreSQL para grupo/team/atleta/sesión/ejercicio/rango exactos, rol `main`, warmup/cooldown excluidos, `wip` excluido, `finished`/`interrupted` aceptados, legacy sin runner_session incluido y legacy sin source_exercise_id resuelto.
- [x] 4.3 Tests de service para entrenador mismo atleta/dos atletas, miembro solo propio, membresía histórica, cada lado fuera de scope, grupo/team cruzados, session ID ajeno, ejercicio que no corresponde y fallo B sin resultado parcial A. Combinados con los tests DAO de membresía/ejercicio/sesión.
- [x] 4.4 Tests del controller para body completo, campo desconocido, IDs faltantes/no positivos, rango inválido, `401/400/403/404/500`, request válida y respuesta JSON con nulos/unidades. `go test ./cmd/api/controllers` pasa.
- [x] 4.5 Ejecutar suite de `dashboard`, `go test ./...`, regenerar Swagger según workflow del repo y revisar `git diff --check`. `go test ./...`, tests de paquetes afectados, `go vet` dirigido y generación Swagger pasan; las pruebas DAO de integración quedan pendientes de Postgres (4.2).

## 5. Cierre de coordinación

- [x] 5.1 Actualizar `paceron-frontend/docs/BACKEND_API_GAPS.md` con endpoint, roles, campos principales de response y estado de disponibilidad (implementado en rama, aún no desplegado).
- [ ] 5.2 Verificar desde frontend con usuario corredor y entrenador: misma persona, dos personas, dos grupos del team, legado y falta de datos; confirmar que no se habilita consumo real antes del endpoint disponible.
