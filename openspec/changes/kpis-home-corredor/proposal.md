# Proposal

## Why

El Home del corredor muestra racha y logros, pero no responde la pregunta más
básica sobre su entrenamiento reciente: **cuánto corrió, a qué velocidad y cuál
fue su mejor marca**. Los datos ya existen en `workout_feedback`, pero hoy el
corredor tiene que entrar al dashboard de un equipo, elegir un grupo y aplicar
filtros para ver un agregado. Ese camino no sirve para el Home por tres razones:

- `team_id` es obligatorio en todos los endpoints de `/dashboard`: el Home no
  tiene un equipo activo y el corredor puede pertenecer a varios.
- El dashboard filtra `session_exercise_instances.role = 'main'`: deja afuera
  calentamientos y vueltas a la calma, y el pedido del Home es sumar **todos**
  los ejercicios del corredor.
- El dashboard está pensado para explorar con filtros, no para una lectura
  rápida con cuatro pestañas y un período calendario.

Agregar el cálculo en el front implicaría duplicar fórmulas que ya viven en el
backend (`KPI_FORMULAS.md`) y abrir una segunda fuente de verdad para velocidad,
tiempo logrado y distancia. El cálculo SHALL seguir en el backend.

## What Changes

- **Un endpoint nuevo, de solo lectura:** `GET /api/v1/dashboard/runner-kpis`.
  No reemplaza ni modifica ninguno de los ocho endpoints existentes.
- **Scope propio, sin `team_id`:** el corredor se identifica por el token. El
  endpoint suma sus series de todos sus grupos y equipos, incluso de grupos de
  los que ya no es miembro, porque el dato es suyo y el Home no tiene un equipo
  como ancla.
- **Período calendario:** `period=month|week|day` y `reference_date` opcional.
  El mes es el mes calendario completo, la semana es la semana ISO (lunes a
  domingo) y el día es la fecha de referencia. La respuesta declara `from`, `to`
  y `days_considered`.
- **Cuatro pestañas, una por request:** `tab=general|ritmo_continuo|velocidad_tiempo|velocidad_distancia`.
  Cada pestaña devuelve solo las métricas que necesita, para que el front pueda
  disparar el cálculo recién cuando el corredor abre esa pestaña.
- **Fórmulas puras y testeadas** en `domains/dashboard`, reutilizando las reglas
  ya existentes de tiempo logrado, velocidad derivada y validez de serie.
- **Sin migraciones, sin columnas nuevas y sin cambios de contrato** en los
  endpoints actuales.

### Objetivo

Que un corredor vea en su Home, en menos de un vistazo y sin elegir equipo ni
grupo, sus números del mes, la semana o el día: distancia, mejor día, mejor
velocidad por serie, velocidad promedio en ritmo continuo y mejores marcas por
tipo de ejercicio.

### Alcance

- Backend: endpoint nuevo, DTO nuevo, servicio y DAO de agregación por corredor,
  validación de período y pestaña, y pruebas unitarias y de contrato HTTP.
- Consumo: el change `kpis-home-corredor` de `paceron-frontend` agrega la
  tarjeta al final del Home autenticado del corredor.
- Los cálculos se hacen sobre series ya registradas. No se crean ni se modifican
  datos de entrenamiento.

### No alcance

- No se modifica el dashboard analítico existente, ni sus endpoints, ni sus
  permisos, ni sus fórmulas.
- No se agrega comparación contra el período anterior, tendencia, proyección ni
  gráficos. El Home pide números, no una segunda pantalla de analítica.
- No se expone el KPI de otro corredor: no hay `athlete_user_id` en el contrato.
- No se agregan índices ni migraciones. La consulta usa el índice existente
  `(athlete_user_id, session_date)` de `workout_feedback`.
- No se calculan calorías, desnivel, cadencia, FC ni RPE. No fueron pedidos y
  sumarían campos sin una regla de producto definida.

### Métrica de éxito

- Un corredor con series en dos grupos distintos ve en `general` la suma de
  ambos, sin cambiar de equipo ni de filtro.
- Cambiar de `month` a `week` o `day` devuelve exactamente el mismo conjunto que
  se obtiene al filtrar esas fechas a mano en la base.
- Las cuatro pestañas devuelven `null` —no `0`— cuando no hay una serie
  calculable, y `0` solo cuando el cero es un resultado real.
- Los endpoints actuales del dashboard siguen respondiendo igual: la suite del
  backend no cambia de comportamiento salvo por los tests nuevos.

## Capabilities

### New Capabilities

- `dashboard-runner-kpis`: endpoint de KPIs del Home del corredor, períodos
  calendario, pestañas y reglas de cálculo.

### Modified Capabilities

Ninguna. El endpoint nuevo reutiliza las fórmulas del dominio del dashboard pero
no cambia ningún requisito existente.

## Impact

- **APIs:** una ruta nueva bajo `/api/v1/dashboard`. Ninguna ruta existente
  cambia de método, parámetros, respuesta ni autorización.
- **Datos:** cero migraciones y cero escrituras. Solo lectura de
  `workout_feedback` y `exercise_instances`.
- **Dependencias:** ninguna nueva.
- **Frontend:** change hermano `kpis-home-corredor` en `paceron-frontend`.
- **Riesgo principal:** que el agregado “de todos los grupos” incluya datos que
  el corredor no esperaba. Se mitiga mostrando el período explícito y no
  aceptando ningún id de corredor: el alcance es siempre el del token.
