# Diseño: nombre explícito del ícono de logro

## Context

El catálogo de logros se define en constantes Go y actualmente usa `icon_key` igual al ID semántico del logro. El frontend pasa ese valor como si fuera un glyph de `MaterialCommunityIcons`, aunque valores como `first_session` o `streak_3` no son nombres de glyph. El frontend necesita recibir la asociación explícita entre cada logro y su ícono.

La respuesta de `GET /api/v1/achievements/catalog` se construye desde `AchievementResponse`; el frontend normaliza el catálogo y lo une con el progreso por `id`. El endpoint de progreso no repite metadata del catálogo. Los íconos disponibles se validaron contra el glyph map de `MaterialCommunityIcons` de `@expo/vector-icons` 15.1.1, que satisface el rango aprobado `^15.0.3`.

## Goals

- Exponer un glyph válido y explícito para cada una de las 21 filas del catálogo.
- Mantener `icon_key` como clave semántica estable igual a `id`.
- Mantener el cambio aditivo, sin migración de DB ni cambios en las reglas de desbloqueo.
- Publicar el nuevo campo en Swagger y en la documentación backend del contrato.

## Non-Goals

- Modificar el frontend, sus componentes, normalizadores o mocks; esos cambios corresponden al repositorio frontend.
- Cambiar IDs, familias, nombres, descripciones, thresholds, unidades o progreso de logros.
- Reemplazar `icon_key`, incluir `icon_name` en el endpoint de progreso o cambiar el tipo de respuesta de stamps.
- Agregar una dependencia de `@expo/vector-icons` al backend o servir assets de íconos.
- Introducir una versión de API nueva o exponer `AchievementCatalogVersion` en el payload.

## Decisions

### D1. Agregar `icon_name` y conservar `icon_key`

`AchievementRow` almacenará ambos valores y `AchievementResponse` expondrá `icon_name` junto a los campos actuales. `icon_key` sigue siendo la identidad semántica del catálogo; `icon_name` es metadata de presentación. El servicio solo agregará el nuevo campo al mapeo de `GET /api/v1/achievements/catalog`.

Alternativas consideradas:

- Reutilizar `icon_key` con nombres de glyph: rechazado porque rompería su semántica actual de ID y acoplaría una clave existente a una presentación.
- Resolver IDs a glyphs en el frontend: rechazado para este contrato porque la asociación quedaría implícita y mantenida por el cliente.
- Enviar una URL de imagen: rechazado porque agrega almacenamiento/serving de assets sin necesidad para un cliente que ya usa glyphs locales.

### D2. Usar glyphs de `MaterialCommunityIcons`

Los valores de `icon_name` serán nombres exactos del glyph map de `MaterialCommunityIcons`, disponibles en `@expo/vector-icons` `^15.0.3`. Se reutilizan glyphs por familia cuando corresponda; la asociación sigue siendo explícita en cada fila.

| `id` | `icon_name` |
|---|---|
| `first_session` | `run` |
| `streak_3` | `fire` |
| `streak_7` | `fire` |
| `streak_14` | `fire` |
| `streak_30` | `fire` |
| `streak_60` | `fire` |
| `distance_10k` | `map-marker-distance` |
| `distance_50k` | `map-marker-distance` |
| `distance_100k` | `map-marker-distance` |
| `distance_42k_single` | `run-fast` |
| `sessions_5` | `calendar-check` |
| `sessions_25` | `calendar-check` |
| `sessions_50` | `calendar-check` |
| `sessions_100` | `calendar-check` |
| `sessions_200` | `calendar-check` |
| `gps_first` | `map-marker-path` |
| `gps_10` | `map-marker-path` |
| `gps_50` | `map-marker-path` |
| `early_5` | `weather-sunset-up` |
| `team_first` | `account-group` |
| `team_10` | `account-multiple-check` |

El backend no importará ni dependerá del paquete frontend. Las pruebas pinnearán el nombre elegido por ID y los valores se revisaron contra el glyph map instalado 15.1.1, dentro del rango `^15.0.3`.

### D3. Incrementar la versión interna del catálogo

`AchievementCatalogVersion` pasará de `v1` a `v2`: agregar `IconName` modifica metadata de cada fila y el comentario fuente exige incrementar la versión cuando se mutan filas. La constante no se expone ni se consume para construir la respuesta; el incremento es interno y no introduce una versión nueva de API. Los IDs, thresholds y reglas siguen iguales.

Alternativa considerada: mantener `v1` porque no cambian IDs ni reglas. Se rechaza para no contradecir la política documentada en el catálogo de que toda mutación de filas incrementa su versión.

### D4. Mantener el cambio en el endpoint de catálogo

El campo se agrega a `AchievementResponse`, que representa filas de `GET /api/v1/achievements/catalog`. `GET /users/{id}/achievements` continúa respondiendo solo progreso; el frontend ya combina ese progreso con la metadata del catálogo por ID. Swagger se regenera desde los DTOs.

## Risks / Trade-offs

- `[Riesgo]` El contrato referencia nombres de una biblioteca visual del frontend → `[Mitigación]` mantener `icon_key` independiente, validar cada glyph contra el paquete dentro del rango aprobado y pinnear la asociación ID→glyph en tests.
- `[Riesgo]` Una versión futura del paquete podría renombrar un glyph → `[Mitigación]` el rango semver `^15.0.3` preserva compatibilidad minor/patch; una migración de set o de nombre se hará como cambio explícito de contrato.
- `[Riesgo]` Una versión vieja del backend no devolverá `icon_name` a un frontend nuevo → `[Mitigación]` el consumidor frontend deberá usar un fallback válido cuando el campo no exista; el despliegue recomendado es backend primero y luego frontend.

## Migration Plan

1. Expandir el catálogo y el DTO con `icon_name`, conservar `icon_key` y cambiar la constante interna a `v2`; no hay migración de datos porque el catálogo vive en código.
2. Regenerar Swagger y actualizar la documentación backend del shape de catálogo.
3. Desplegar primero el backend: clientes antiguos ignoran el campo adicional; luego el frontend puede consumir `icon_name` con un fallback defensivo.
4. Para rollback del backend, revertir el campo y la constante a `v1`. El frontend deberá conservar el fallback durante el rollout mixto. No se elimina `icon_key` en este cambio.
