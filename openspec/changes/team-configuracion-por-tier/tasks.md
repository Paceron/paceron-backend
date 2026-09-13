## 1. Domain (`cmd/api/domains/teamconfiguration`)

- [x] 1.1 Crear `team_configuration.go`: struct `TeamConfiguration` (`MaxMembers int json:"max_members"`, `MinimumFee float64 json:"minimum_fee"`), hashmap `teamTierConfigurations` (`base` 10/20000, `medium` 25/20000, `premium` 50/20000), constants `DefaultMaxMembers = 10` y `DefaultMinimumFee = 20000`, y `ForTier(tierName string) TeamConfiguration` con fallback al default.
- [x] 1.2 Test unit `team_configuration_test.go`: `ForTier("base"/"medium"/"premium")` devuelve lo del mapa; `ForTier("desconocido")` y `ForTier("")` devuelven el default.

## 2. Service (`cmd/api/services/team_configuration_service.go`)

- [x] 2.1 Interfaz `TeamConfigurationServiceInterface.GetTeamConfiguration(ctx, userID int64) (*teamconfiguration.TeamConfiguration, error)`.
- [x] 2.2 Implementación: resolver rol "entrenador" + tier (sub vigente → `user_roles.tier_id` → default) y devolver `ForTier(tierName)`. Sin dependencia de `TeamDao` ni validación de equipo.

## 3. Controller (`cmd/api/controllers/team_configuration_controller.go`)

- [x] 3.1 Interfaz `TeamConfigurationController.GetTeamConfiguration(c *gin.Context)`: identidad desde el access token (`utils.GetAuthUserID`, `401` si no hay), error mapping solo `500`, respuesta JSON 200.
- [x] 3.2 Ruta `GET /api/v1/team-configuration` en `url_mappings.go` + wiring en `app.go`.

## 4. Swagger

- [x] 4.1 Godoc del endpoint sin params (identidad del access token).
- [x] 4.2 Regenerar y verificar en `swagger.json`/`swagger.yaml` que el endpoint y el schema `TeamConfiguration` aparecen.

## 5. Tests

- [x] 5.1 `team_configuration_service_test.go`: sub vigente (premium) → config del mapa; sin sub con `user_roles.tier_id` (base); rol entrenador inexistente → default; sin sub ni user_role → default; tier desconocido → default.
- [x] 5.2 `team_configuration_controller_test.go`: success (200 + JSON plano, userID del token), sin identidad → 401.
- [x] 5.3 `go test ./...` en verde y coverage con DB (CI) sin bajar del gate (≥ 80%).

## 6. Documentación

- [x] 6.1 Agregar la fila del endpoint en la tabla de `README.md`.
- [x] 6.2 (`openspec validate`) spec válida y tasks marcadas al final.