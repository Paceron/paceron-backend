# Agregar nombres de ícono al catálogo de logros

## Objetivo

Exponer en el catálogo de logros un nombre de glyph explícito que el frontend pueda pasar a `MaterialCommunityIcons`, sin inferirlo desde el ID ni cambiar la identidad semántica del logro.

## Alcance

- Agregar `icon_name` como campo obligatorio de cada fila de `GET /api/v1/achievements/catalog`.
- Mantener `icon_key` y su valor actual igual al ID del logro.
- Asignar a las 21 filas un glyph de `MaterialCommunityIcons` compatible con `@expo/vector-icons ^15.0.3`.
- Propagar el campo desde las constantes del catálogo al DTO de respuesta y a Swagger.
- Actualizar la documentación backend del contrato y su impacto previsto en el frontend.

## No alcance

- Cambiar IDs, familias, thresholds o reglas de desbloqueo, ni exponer un nuevo versionado en la API.
- Quitar o reinterpretar `icon_key`, agregar `icon_name` al endpoint de progreso o cambiar el contrato de estampas.
- Implementar cambios en el repositorio frontend.
- Agregar tablas, migraciones o una dependencia de íconos al backend.

## Métrica de éxito

- El catálogo devuelve 21 filas y cada una incluye un `icon_name` no vacío, validado contra el glyph map de `MaterialCommunityIcons` de `@expo/vector-icons ^15.0.3`.
- `icon_key`, IDs, thresholds y progreso conservan su comportamiento actual.
- Las pruebas de catálogo y el Swagger generado reflejan el campo nuevo; `go test ./...` y `openspec validate achievement-icon-name --strict` pasan.

## Why

Los valores actuales de `icon_key` identifican semánticamente cada logro y no son nombres de glyphs. El frontend no debe adivinar qué imagen corresponde a cada ID ni pasar esos valores directamente a una API de íconos. El catálogo backend ya es la fuente de metadata de presentación, así que puede entregar la asociación de forma explícita.

## What Changes

Se amplía el contrato de `GET /api/v1/achievements/catalog` con `icon_name`, un nombre exacto de glyph de `MaterialCommunityIcons`. El campo se agrega a cada fila del catálogo Go y a `AchievementResponse`; `icon_key` permanece intacto. El `AchievementCatalogVersion` interno sube de `v1` a `v2` porque cada fila agrega metadata, según la regla del catálogo. La constante no se expone ni se consume en el API, así que el payload sigue siendo aditivo y no varían IDs, thresholds ni reglas de desbloqueo. Se actualizan tests, Swagger y documentación de impacto.

## Capabilities

### New Capabilities

- `gamification-achievements`: completar el contrato del catálogo con un nombre de glyph explícito para cada logro. La capability estaba definida en el change no archivado `gamificacion-core`; el nuevo spec volverá a declarar el requisito completo bajo `ADDED` y aclarará que extiende el shape anterior, que solo incluía `icon_key`.

## Impact

- Backend: constantes del catálogo, DTO de gamificación, mapper del servicio, pruebas de catálogo y documentación Swagger generada.
- Documentación: `docs/CATALOGO_Y_CALENDARIO.md` y `docs/FRONTEND_IMPACTO_INSTANCIACION.md` reflejarán el campo aditivo.
- Compatibilidad: consumidores existentes pueden ignorar el campo nuevo; `icon_key` se conserva. El frontend podrá adoptarlo en un cambio separado.
- Persistencia: no requiere migración de base de datos.
