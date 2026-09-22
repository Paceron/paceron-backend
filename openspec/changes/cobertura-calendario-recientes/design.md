# Design: cobertura-calendario-recientes

## Context

Ver proposal.md. El perfil de coverage (dedup del `-coverpkg` doble instrumentación con awk: max count por location) da: total 81.75% (7411/9065). Faltantes en el área reciente: `calendar_service.go` 173, `session_service.go` 37, `exercise_service.go` 25, `calendar_controller.go` 20, DAOs de calendario/instancias/grupos ~31 (subtotal ~285). Meta: ≥ 84% total, gate 80 intacto.

## Goals / Non-Goals

**Goals:**
- Cubrir statements alcanzables del área reciente con tests reales (Postgres para DAO/servicio, mock para controller).
- Helper reutilizable `FailingDB` en `testutils` para ramas de error de DB (`fmt.Errorf("...: %w", err)`, `customlogger.Error`).
- Llevar el total a ≥ 84% sin bajar el umbral ni tocar `.testcoverage.yml`.

**Non-Goals:**
- Etapa 2 (legacy: mercadopagoclient, customlogger, payment/user/workout_feedback services) — registrada como pendiente en `docs/DEUDA_TECNICA_Y_PENDIENTES.md`.
- Cambiar lógica de producción (ningún `.go` fuera de `testutils` se modifica).
- Cubrir al 100%: ramas no falsificables legítimamente quedan parked con ruling (documentadas en tasks/design).

## Decisions

**D1 — Helper `FailingDB` en `testutils` (generalización del patrón existente).**
El test de colisión de Shift ya usa un handle `*gorm.DB` con condición de test para forzar un fallo puntual. Se extrae a `testutils.FailingDB(t, db, cond func(operation string) bool)` que envuelve el handle y devuelve error de Postgres para las operaciones que la condición marca. Alternativa descartada: burlar el DAO entero (los tests de services usan Postgres real; mocks romperían el valor de las pruebas). Las ramas de error así cubiertas son las envolturas `%w` y el logging de `customlogger.Error` — comportamiento observable ante falla de DB.

**D2 — Prioridad de escenarios reales antes que helper de fallos.**
Primero lo alcanzable con Postgres real o mocks existentes (sin infraestructura nueva): catálogo borrado al asignar (`ErrCalendarSessionNotFound`), link sin ejercicio, variantes de `validateDayFields` (rest/other/training con y sin session_id/instancia, presencial con horarios inválidos), conflictos de stamp, banner con instancia nil, grupo sin nombre, paths mock `s.db == nil` (stamp vacío, Shift invertido, "no hay DB disponible"). El helper FailingDB entra en una tarea posterior para las envolturas de error que no se pueden provocar con datos.

**D3 — Clasificación de no-falsificables.**
`presencialTimeHHMM` ya cubierto; ramas defensivas de deserialización/parse interno no falsificables (p. ej. hooks internos de GORM que nunca devuelven nil error y a la vez falla JSON) se dejan y se anotan en el reporte con su línea. Criterio: si cubrirlo requiere romper el test o simular el runtime (no la dependencia), es parked.

**D4 — Controller con mock (convención del paquete).**
`calendar_controller.go` (~20 missing): ramas restantes de handlers con `mockCalendarService` existente; sin DB real. Verificación por status code + body.

**D5 — Orden de verificación del número.**
`go clean -cache` antes de `make coverage-with-db` (bug conocido de cache con `-coverpkg`): número raw ~81.8% y analyzer 82.1% sobre la misma base; la comparación de progreso se hace SIEMPRE con el analyzer tras clean.

## Risks / Trade-offs

- El helper FailingDB acopla tests al pattern de firma de los DAOs (si cambia el handle, se rompe el helper) — costo bajo, es testutils.
- Coverage tiene techo natural: statements defensivos no falsificables dejan el ~90% alcanzable solo con etapa 2; esta etapa apunta a 84-84.5%.
- Los tests de error de DB pueden volverse frágiles si se acoplan a mensajes de error: la condición se evalúa por tipo de operación, no por texto.
