## 1. Modelo

- [x] 1.1 `domains/dbs/team.go`: campo `ShowGroupsToRunners bool` (default `false`, columna aditiva vía AutoMigrate)

## 2. DTOs

- [x] 2.1 `CreateTeamRequest`/`UpdateTeamRequest`: `ShowGroupsToRunners *bool` opcional
- [x] 2.2 `TeamResponse`: `ShowGroupsToRunners bool`

## 3. Service

- [x] 3.1 `Create`: aplica el valor si viene, default `false` si no
- [x] 3.2 `Update`: aplica el valor si viene
- [x] 3.3 `toResponse`: expone el campo
- [x] 3.4 Tests: create con/sin el campo, update

## 4. Documentación

- [x] 4.1 Regenerar Swagger
- [x] 4.2 Request Bruno (`23_actualizar_visibilidad_grupos.yml`)

## 5. Verificación

- [x] 5.1 `go build ./...`, `go vet ./...`, `go test ./...` — todo verde
- [ ] 5.2 Probar manualmente: crear equipo con el campo, actualizar equipo seteándolo, confirmar que persiste en `GET /teams/:id`
