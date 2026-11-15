# Tasks

## Restricciones globales

- Mantener contrato HTTP/DTO/JSON/status/auth/orden/timezone/errores/soft-delete.
- No frontend, Swagger, schema, índices ni migraciones.
- Tests de paridad y query-count; coverage total ≥85%.
- Stagear rutas explícitas; no `.superpowers/`; commit/push al terminar; el usuario mergea.

## 1. Auth de calendario

- [x] 1.1 Agregar `GroupDao.FindWithTeamOwnerByID` con `LEFT JOIN teams` y usarlo en `isGroupOwner`/`isGroupOwnerOrMember`, preservando `ErrCalendarGroupNotFound`, `ErrCalendarForbidden` y el error genérico de DB. Verificar con `go test ./cmd/api/services -run 'TestCalendarService.*(Owner|Member|Auth|Error)'` y tests de 404/403/500.
- [x] 1.2 Adaptar `mockGroupDao` y agregar query-count con DB real para `GetRange` owner/miembro y `UpsertDay`; confirmar 5→4/6→5 y 6→5. Verificar con `TEST_DB_* go test ./cmd/api/services -run 'TestCalendarService.*QueryCount|TestCalendarService_UpsertDay'`.

## 2. Roster de equipo

- [x] 2.1 En `team_user_service.GetUsersByTeam`, autorizar al caller desde el roster (`FindByTeamID`) y eliminar `FindByTeamAndUser`; preservar 404 de equipo, 403 de no-miembro y el shape/orden del roster. Verificar con `go test ./cmd/api/services -run TestTeamUserService_GetUsersByTeam`.
- [x] 2.2 Adaptar/eliminar el test que fuerza error de la lectura dedicada y agregar query-count con DB real (2 queries, independiente de N). Verificar con `TEST_DB_* go test ./cmd/api/services -run TestTeamUserService_GetUsersByTeam`.

## 3. Colisiones presenciales y listado de equipos

- [x] 3.1 En `findPresencialCollisions`, usar `FindByOwnerIDWithTeamName` para grupos y nombres de equipo; preservar conflictos/orden/errores. Verificar con `go test ./cmd/api/services -run 'TestCalendarService.*Presencial|TestDBError_findPresencialCollisions'` y query-count.
- [x] 3.2 Agregar `TeamDao` con `owner_id` + `member_id` vía `EXISTS` y usarlo en `GetAll` cuando ambos filtros están presentes; preservar shape/orden y evitar duplicados. Verificar con `go test ./cmd/api/services -run TestTeamService_GetAll` y test de dos membresías activas.

## 4. Verificación final

- [x] 4.1 Ejecutar `TEST_DB_* go test -count=1 ./...`, `go vet ./...`, `go build ./...`, `openspec validate optimizaciones-equipos-auth-calendario --strict` y `git diff --check`; confirmar todo verde.
- [x] 4.2 Medir coverage CI-equivalente y confirmar ≥85%; documentar resultado.
