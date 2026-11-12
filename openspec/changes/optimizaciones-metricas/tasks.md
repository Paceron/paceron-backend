# Tasks

## Restricciones globales

- No cambiar endpoints, parámetros, DTOs, tags JSON, status HTTP, autorización ni payloads WS; no modificar `paceron-frontend`.
- La única corrección de resultado permitida es la atribución de `gps_N` al atleta dueño del feedback indicada por la spec delta; no revocar progreso ya guardado.
- Preservar timezone `America/Argentina/Cordoba`, orden, errores y semántica de soft-delete salvo donde la spec lo precisa.
- Tests con PostgreSQL de test: `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`; nunca ejecutar `local-*`, `demo-*` ni `fullseed-*`.
- Coverage gate 85: no modificar `.testcoverage.yml`; al final usar `go clean -cache`, `make coverage-with-db` y el analyzer `go run github.com/vladopajic/go-test-coverage/v2@latest --config ./.testcoverage.yml --profile ci/test_coverage/coverage.out`.
- No agregar índices ni cachés entre requests sin evidencia y diseño de invalidación; no tocar pagos ni `/auth/user`.
- Stagear solo rutas explícitas, nunca `git add -A`/`git add .`; no commitear `.superpowers/`; no push/merge.
- Comentarios de código mínimos; Swagger no se regenera si no cambian anotaciones/DTOs.

## Etapa 1 — Exactitud y CPU del dashboard

### Task 1: Atribución correcta de puntos GPS en logros

- [ ] 1.1 Cambiar `CountGPSFinishedSessions` para asociar los puntos al feedback y al atleta evaluado; agregar test PostgreSQL de dos atletas en una instancia compartida, incluyendo feedback soft-deleted según la spec.
- [ ] 1.2 Verificar que `gps_first`, `gps_10` y `gps_50` conservan el mismo contrato y que una re-evaluación sigue siendo idempotente; correr tests DAO/service del logro GPS.

### Task 2: Dashboard sin agregaciones descartables ni validación repetida

- [ ] 2.1 Extraer el cálculo puro de `DataQuality` reutilizando las reglas actuales y probar paridad contra `BuildSummary(...).DataQuality` con inválidos, legacy, series insuficientes, velocidades y sesiones abiertas.
- [ ] 2.2 Usar el cálculo ligero en Leaderboard, ExerciseSeries, Comparisons, Periods y TimeSeries; eliminar la segunda validación idéntica de atletas en Comparisons y probar igualdad de respuesta y una sola lectura del roster.
- [ ] 2.3 En `resolveTeamGroup`, reutilizar el grupo presente en `DashboardScope` y consultar el DAO solo para grupos fuera del scope; probar que 403/404 y el orden de validaciones permanecen iguales.

### Task 3: Evaluar y, solo con paridad, optimizar `ScopeDateRange`

- [ ] 3.1 Agregar fixture amplio y prueba de referencia que fije `MIN/MAX` con los mismos filtros de series, incluidos rol main, runner status, membresía y feedback huérfano.
- [ ] 3.2 Medir la consulta actual y la alternativa con el mismo fixture (`EXPLAIN (ANALYZE, BUFFERS)` o conteo/benchmark); aplicar una reescritura solo si reduce el trabajo y devuelve el mismo rango, de lo contrario dejar SQL intacto y registrar la medición como deuda.

## Etapa 2 — Competencias y evaluadores de logros

### Task 4: Reutilizar agregaciones durante el cierre de competencias

- [ ] 4.1 Reusar el agregado de métricas ya obtenido por Close para winners y respuesta; evitar listar intentos en competencias metric, y compartir las lecturas route ya obtenidas dentro del request cuando no cambie el snapshot lógico.
- [ ] 4.2 Agregar tests de paridad de winners/progress/stamps/unseen y query-count para Close metric y route, incluidos empate exacto y competencia sin intentos.

### Task 5: Agrupar métricas e inserciones del evaluador de logros

- [ ] 5.1 Combinar consultas de runner sessions que sirven a `sessions_N`/`early_N` y de feedback que sirven a distancia total/máxima por sesión; probar paridad en límites de hora, timezone Cordoba, feedback borrado y múltiples sesiones.
- [ ] 5.2 Agregar inserción batch idempotente de unlocks que reporte solo los IDs insertados; mantener orden de catálogo y emitir unseen/push únicamente para nuevos logros.
- [ ] 5.3 Probar primera evaluación, segunda evaluación idempotente, salto de varios thresholds, error parcial y query-count; no debilitar tests de hooks existentes.

### Task 6: Evaluar asistencia bulk en lote

- [ ] 6.1 Agregar agregación de días de asistencia para varios usuarios en una sola lectura, con los mismos filtros y `COUNT(DISTINCT gcd.date)` actuales; probar con varios equipos/días y sin filas.
- [ ] 6.2 Cambiar el hook de BulkSaveAttendance para encolar una sola evaluación por lote y procesar resultados por usuario con concurrencia acotada; conservar la re-evaluación de filas insertadas y actualizadas, unseen/push idempotentes y frames WS por fila.
- [ ] 6.3 Probar query-count constante para roster pequeño/grande, resultado equivalente por usuario, `-race`, error del agregado y nil-safe del hook.

## Etapa 3 — Escrituras y perfil GPS

### Task 7: Borrar un día con el camino batch

- [ ] 7.1 Migrar DeleteDay al borrado de instancias en lote dentro de la misma transacción; probar contenido, orden, colisión, rollback y preservación D10 con feedback de sesión/ejercicio.

### Task 8: Reducir la lectura del recompute de puntos GPS

- [ ] 8.1 En el recompute completo de points, seleccionar solo `order`, `recorded_at`, `latitude` y `longitude`; probar que pace/peak siguen idénticos para lotes parciales, repetidos y puntos fuera de orden.

## Etapa 4 — Documentación y verificación

### Task 9: Cerrar documentación y gates

- [ ] 9.1 Actualizar `docs/DEUDA_TECNICA_Y_PENDIENTES.md` con hallazgos resueltos y los que quedan diferidos; no duplicar deuda existente.
- [ ] 9.2 Ejecutar `openspec validate optimizaciones-metricas --strict`, gofmt de archivos Go tocados, `go build ./...`, `go vet ./...` y `TEST_DB_* go test -count=1 ./...`.
- [ ] 9.3 Ejecutar coverage con cache limpio y analyzer; comprobar gate 85, `.testcoverage.yml` intacto, `git diff --check` y todos los checks de tareas completados antes del review final.
