## 1. Creación de equipo

- [x] 1.1 `delegates/team_delegate.go`: invertir condición — crear grupo default salvo `create_default_group: false` explícito
- [x] 1.2 Tests (`team_delegate_test.go`, paquete sin tests previos): flag omitido, `true`, `false`, error de team create

## 2. Alta directa de usuario

- [x] 2.1 `services/team_user_service.go`: `TeamUserService` gana `groupDao`/`groupUserDao`
- [x] 2.2 `AddUser` llama `assignToMainGroup` tras crear el `TeamUser` (no bloqueante)
- [x] 2.3 Tests: asigna al grupo principal, sin grupo principal sigue funcionando

## 3. Wiring y documentación

- [x] 3.1 `app/app.go`: `NewTeamUserService` recibe `groupDao`/`groupUserDao`
- [x] 3.2 Doc de `create_default_group` en `domains/team/team_request.go` actualizada
- [x] 3.3 Regenerar Swagger

## 4. Verificación

- [x] 4.1 `go build ./...`, `go vet ./...`, `go test ./...` — todo verde
- [ ] 4.2 Probar manualmente: crear equipo sin el flag → tiene grupo; `POST /teams/:id/users` → usuario aparece en `GET /groups/:id/users` del grupo principal
- [ ] 4.3 Comunicar al frontend el cambio de default de `create_default_group`
