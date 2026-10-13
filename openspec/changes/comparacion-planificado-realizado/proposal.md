# Propuesta: Comparación planificado vs realizado

## Why

El corredor registra cada serie que corre (`workout_feedback`), y el entrenador
planifica cada día con su propia copia de la sesión (`session_instances` →
`session_exercise_instances` → `exercise_instances`). Pero nada cruza las dos
cosas: el corredor no puede saber si cumplió lo que tenía planificado (HU
"Comparar planificado vs realizado").

Además, el frontend no puede armar la comparación por su cuenta: el detalle de
la sesión que recibe (`GET /session-instances/:id`) trae solo `repeat_count`,
`role` y `rest_minutes` de cada ejercicio. Los objetivos (minutos, distancia,
velocidad, tipo, intensidad) solo están en `exercise_instances`, que hoy no
expone ningún endpoint para el corredor.

## Objetivo

Que un corredor autenticado pueda consultar:

1. Para una sesión: lo planificado contra lo realizado, ejercicio por ejercicio y
   serie por serie, con la desviación de tiempo y distancia, el ritmo, y un
   porcentaje de cumplimiento de la sesión.
2. Para un rango de fechas: el cumplimiento de cada sesión planificada (también
   las que no hizo) y un resumen del período, con filtros por equipo, grupo y
   tipo de entrenamiento.

Que un entrenador pueda consultar lo mismo para un corredor de un equipo suyo.

## Alcance

- Dos endpoints `GET` autenticados:
  - `/api/v1/activities/comparison/:sessionInstanceId`: detalle de una sesión.
  - `/api/v1/activities/comparison`: lista por rango de fechas con resumen.
- Funciones puras de cálculo de cumplimiento y desviación.
- Un DAO de solo lectura (`activity_comparison_dao.go`) con las consultas de
  días planificados, ejercicios planificados, feedback y `runner_session`.
- Service con las reglas de acceso, la validación de parámetros y el armado de
  la respuesta.
- Controller, wiring en `app.go`, rutas y swagger.
- Tests de cálculo, DAO, service y controller.
- Registro en `docs/DEUDA_TECNICA_Y_PENDIENTES.md` de lo que se detectó.

## No alcance

- Comparar por plan de entrenamiento (`training_plans`): el vínculo día → plan
  (`group_calendar_days.source_plan_id`) se pierde cuando el entrenador edita el
  día, así que no es confiable.
- Exportar a PDF o CSV: lo arma el frontend con la respuesta.
- Gráficos: el frontend los dibuja con la lista.
- Comparar ritmo cardíaco, cadencia o desnivel: no tienen objetivo planificado.
- Persistir el cumplimiento calculado: se calcula en cada consulta.
- Migrar las filas viejas de `workout_feedback` con `set_number` en base 0: se
  normalizan al leer.

## Métrica de éxito

- Para la ejecución sembrada del 2026-10-12 (sesión 455, corredor 19: primera
  caminata completa, segunda al 50%, tercera salteada) el detalle devuelve
  100%, 50% y 0% por ejercicio y **50%** para la sesión.
- Los archivos nuevos quedan con 90% o más de cobertura y el total del repo se
  mantiene en 85% o más.
