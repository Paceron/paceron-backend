## 1. DAO

- [x] 1.1 `daos/team_dao.go`: `GetAllByOwnerID`, `GetAllByMemberID` (join con `team_users`)

## 2. Service

- [x] 2.1 `TeamServiceInterface.GetAll` cambia de firma: `(ctx, ownerID *int64, memberID *int64)`
- [x] 2.2 Combinación de ambos filtros resuelta en memoria (`filterByMember`)
- [x] 2.3 Tests: sin filtros, solo owner, solo member, ambos, error de DAO

## 3. Controller

- [x] 3.1 Parseo de `owner_id`/`member_id` como query params opcionales, mismo patrón que `group_controller.GetAll`
- [x] 3.2 Tests: sin filtros, con cada filtro, ID inválido (400)

## 4. Documentación

- [x] 4.1 Regenerar Swagger
- [x] 4.2 Actualizar tabla de endpoints en `README.md`
- [x] 4.3 Requests Bruno (`21_obtener_equipos_por_owner.yml`, `22_obtener_equipos_por_member.yml`)

## 5. Verificación

- [x] 5.1 `go build ./...`, `go vet ./...`, `go test ./...` — todo verde
- [ ] 5.2 Probar manualmente: `GET /teams?owner_id=X`, `GET /teams?member_id=Y`, sin filtros
