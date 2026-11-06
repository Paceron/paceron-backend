# KPIs del Home del entrenador

## Why

El Home del entrenador no resume actividad, equipos, corredores ni cobranzas. El
entrenador necesita una vista inicial útil sin entrar equipo por equipo. El
resumen debe compartir los períodos calendario Mes/Semana/Día del Home del
corredor, y recibir datos agregados desde el backend.

## What Changes

- Agregar `GET /api/v1/dashboard/trainer-home-kpis`, autenticado y acotado al
  entrenador del token.
- Devolver sesiones programadas/realizadas, sesiones presenciales
  programadas/realizadas, equipos y corredores activos, cobrado neto y cuotas
  pendientes para el período seleccionado.
- Usar los períodos `month|week|day` y la misma regla de período de
  `runner-kpis` (mes calendario, semana lunes-domingo, día de referencia).
- No modificar el schema ni cambiar endpoints existentes.

## Decisiones confirmadas

- Una sesión está programada si hay un día `training` con instancia en el
  calendario del período y no está cancelado.
- Una sesión se considera realizada solo si tiene al menos una asistencia
  registrada. El tooltip debe explicitar esta regla.
- Las sesiones presenciales son un subconjunto y reportan también
  programadas/realizadas.
- “Cobrado neto” son pagos aprobados menos la comisión de Paceron. “Pendiente”
  son cuotas cuyo vencimiento cae dentro del período.
- Equipos/corredores son cifras actuales (no cambian al elegir período). Los
  corredores se cuentan únicos entre los equipos activos del entrenador.

## Capabilities

### New Capabilities
- `trainer-home-kpis`: resumen agregado del Home de entrenador, scoped por token.

### Modified Capabilities
- None.

## Impact

- Controller, service, DAO, DTO y ruta del dashboard.
- Tests unitarios y de integración/DAO, documentación Swagger.
- Sin cambios de esquema ni a contratos existentes.
