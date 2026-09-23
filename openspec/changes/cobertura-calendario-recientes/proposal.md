# Propuesta: cobertura-calendario-recientes

## Why

El gate de coverage del repo es 80% y estamos en ~82.1%. El usuario fijó 90% como meta y 85% como objetivo inmediato; el mayor margen mejorable sin tocar deuda legacy está en el área trabajada recientemente (calendario, sesiones, ejercicios y sus DAOs/controllers), donde ~285 statements siguen sin cubrir.

## What Changes

- Tests exhaustivos para el área recientemente trabajada (etapa 1 del plan de cobertura):
  - `calendar_service.go` (~173 missing): escenarios reales alcanzables (catálogo borrado al asignar, link sin ejercicio, variantes de `validateDayFields`, conflictos de stamp, banner con instancia nil, grupo sin nombre, paths mock `s.db == nil` restantes) y ramas `fmt.Errorf("...: %w", err)` tras llamadas DAO.
  - `session_service.go` (~37), `exercise_service.go` (~25), `calendar_controller.go` (~20) y DAOs de calendario/instancias/grupos (~31).
  - Helper `FailingDB` en `cmd/api/testutils` (handle gorm con condición de test, patrón ya usado en el test de colisión de Shift) para falsificar errores de DB en ramas de error de servicio/DAO.
- **BREAKING**: ninguno. Cambio 100% de tests + helper de testutils.
- Resultado esperado: total ≥ 84-84.5% (go-test-coverage), sin bajar el gate de 80 ni tocar `.testcoverage.yml`.
- Las ramas no falsificables legítimamente quedan parked con ruling en el ledger.

## Capabilities

- **New Capabilities**: ninguna.
- **Modified Capabilities**: ninguna (no cambia comportamiento observable; solo tests).

## Impact

- Código: solo `*_test.go` y `cmd/api/testutils` (helper `FailingDB`).
- CI: `make coverage-with-db` debe seguir en verde con margen mayor.
- Deuda registrada aparte: etapa 2 (legacy: mercadopago, customlogger, payment, user, workout_feedback → 90%) queda pendiente en `docs/DEUDA_TECNICA_Y_PENDIENTES.md` y sirve como trabajo de repuesto cuando no haya gaps/trabajos nuevos.
