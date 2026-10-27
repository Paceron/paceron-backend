# Proposal

## Why

El dashboard puede comparar períodos y corredores, pero no dos ejecuciones concretas de ejercicios. Para saber si una sesión B mejoró respecto de una A, el cliente necesitaría combinar filas de feedback y repetir cálculos de rendimiento que deben seguir viviendo en el backend.

## What Changes

- Agregar `POST /api/v1/dashboard/comparisons/sessions` para comparar dos referencias de sesión/ejercicio dentro de un team y rango de fechas.
- Devolver para cada lado series registradas/válidas, distancia total, velocidad media, RPE medio, mejor/peor serie según el `kind` y las series necesarias para graficar.
- Devolver diferencias B−A con unidades declaradas, tratando `null` como dato no calculable y no como cero.
- Validar de forma independiente el scope, grupo, corredor, sesión y ejercicio de A y B; el modo corredor solo permite sus propias sesiones y el entrenador puede comparar uno o dos corredores de su scope.
- Reusar el catálogo y las instancias congeladas; resolver referencias legacy por nombre normalizado, sin migración de schema.

## Capabilities

### New Capabilities
- `dashboard-session-comparison`: comparación de dos ejecuciones por ejercicio, con métricas tipificadas, series por repetición y autorización por cada lado.

### Modified Capabilities
- Ninguna. `dashboard-analytics` no existe todavía en `openspec/specs/`; este comportamiento se define como una capacidad acotada y nueva.

## Objetivo

Entregar al frontend un resultado A/B completo y seguro para comparar ejercicios de sesiones reales, sin duplicar fórmulas de KPI en el cliente.

## Alcance

Dos lados A/B con team, grupo, corredor, `session_instance_id` y ejercicio de catálogo. Se admiten roles principales (`main`) de sesiones `finished`/`interrupted` y feedback histórico sin `runner_session`; se excluyen `wip`, `warmup` y `cooldown`.

## No alcance

- No se agregan tablas, columnas, índices ni nuevas dependencias.
- No se implementa la UI ni se cambian los endpoints de comparación de períodos o entre corredores.
- No se comparan planes prescritos sin feedback ejecutado.

## Métrica de éxito

- Un entrenador puede comparar dos sesiones dentro de su scope o dos corredores distintos; un corredor solo sus propias sesiones.
- Mejor/peor se interpreta por tipo: tiempo y distancia para `ritmo_continuo`, velocidad para `velocidad_distancia` y distancia para `velocidad_tiempo`.
- Los totales/deltas y valores por serie vienen calculados por la API; el frontend no reproduce fórmulas.
- Un grupo, corredor, sesión o ejercicio ajeno se rechaza sin filtrar datos de A o B.

## Impact

- **Controller/ruta:** nuevo handler autenticado de comparación de sesiones y anotaciones Swagger.
- **Service/DAO:** validación del scope de ambos lados y consulta de series persistidas con objetivo congelado y rol de sesión.
- **DTOs:** request/ref por lado, resumen por lado, delta y puntos por serie.
- **Base de datos:** sin cambios de schema; se reusan las relaciones e índices actuales.
