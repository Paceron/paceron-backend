## 1. Mapeo del catálogo y contrato API

- [x] 1.1 Añadir pruebas RED: extender `TestAchievementsService_GetCatalog` para inspeccionar el JSON serializado y afirmar las 21 asignaciones `id → icon_name` definidas en `design.md` y que `icon_key` siga igual a `id`, sin referenciar un campo DTO aún inexistente. Ejecutar `go test ./cmd/api/services -run TestAchievementsService_GetCatalog` y confirmar que RED se debe a la ausencia de `icon_name` en el JSON, sin errores de compilación.
- [x] 1.2 Agregar `IconName` a `AchievementRow`, asignar los glyphs de `design.md` a las 21 filas y subir `AchievementCatalogVersion` de `v1` a `v2`; agregar `IconName` a `AchievementResponse` y mapearlo en `toAchievementResponse`, sin cambiar `IconKey`. Verificar con `go test ./cmd/api/constants ./cmd/api/services ./cmd/api/controllers`.

## 2. Documentación y contrato publicado

- [x] 2.1 Actualizar `docs/CATALOGO_Y_CALENDARIO.md` §8.14 y `docs/FRONTEND_IMPACTO_INSTANCIACION.md` §16.3 para documentar `icon_name`, su uso como glyph de `MaterialCommunityIcons` y que el shape del endpoint de progreso no cambia. Verificar que los ejemplos incluyan el campo nuevo y conserven `icon_key`.
- [x] 2.2 Regenerar Swagger con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`; comprobar que `icon_name` aparece en `AchievementResponse` en `cmd/api/docs/docs.go`, `cmd/api/docs/swagger.json` y `cmd/api/docs/swagger.yaml`.

## 3. Verificación final

- [x] 3.1 Ejecutar `go test ./...`, `openspec validate achievement-icon-name --strict` y `git diff --check`; confirmar que la suite pase, OpenSpec sea válido y el diff no tenga errores de whitespace.
