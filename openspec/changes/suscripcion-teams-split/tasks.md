# Tasks

## Tasks

### 1. Modelos de DB y migración

- [ ] Agregar `membership_fee numeric NOT NULL DEFAULT 0` a `cmd/api/domains/dbs/team.go`.
- [ ] Agregar `subscription_status`, `init_amount`, `paid_installments` a `cmd/api/domains/dbs/team_user.go`.
- [ ] Crear `cmd/api/domains/dbs/seller_connection.go` (D6: user_id único, mp_user_id, tokens, status).
- [ ] Crear `cmd/api/domains/dbs/platform_setting.go` (key-value: key, value JSONB, updated_by, timestamps).
- [ ] Registrar en AutoMigrate (`cmd/api/infrastructure/postgresdb/postgres.go`).
- [ ] Validar con `go build ./...` y test de DAO con Postgres real.

### 2. DAOs

- [ ] Crear `cmd/api/daos/seller_connection_dao.go`: `Upsert`, `FindByUser`, `SetStatus`, `FindAuthorizedByUser`.
- [ ] Crear `cmd/api/daos/platform_setting_dao.go`: `Get` (default si no existe), `Set`.
- [ ] Extender `cmd/api/daos/installment_dao.go`: `FindPendingByUserTeam`, `FindNextByUserTeam`.
- [ ] Extender `cmd/api/daos/team_user_dao.go` con métodos de suscripción (incrementar `paid_installments`, activar, estado).
- [ ] Tests unitarios (mocks) y de DAO con Postgres real.

### 3. DTOs y dominios

- [ ] Crear `cmd/api/domains/teambio/`: request/response de estado de cuenta (shape D3).
- [ ] Crear `cmd/api/domains/mpconnect/`: request/response connect/callback/status.
- [ ] Crear `cmd/api/domains/platformsettings/`: request/response de `marketplace-fee`.
- [ ] `domains/payment/`: constante `concept = team_subscription`; propagar `installment_id` (base del change 1).
- [ ] Custom codes de error: `SELLER_NOT_CONNECTED`, `TEAM_DEBT_BLOCKS_OPERATION`, `NOT_APP_OWNER`.

### 4. Servicios

- [ ] `services/team_user_service.go` AddUser: gate D2 (transacción membership + cuota #1 si `membership_fee > 0`).
- [ ] `services/invitation_service.go` AcceptInvitation: mismo gate (reutilizar helper).
- [ ] `services/team_user_service.go` RemoveUser: bloqueo por deuda (D4) + soft-delete.
- [ ] Crear `services/team_subscription_service.go`: `GetTeamSubscription` (D3) y lógica de generación de cuota siguiente.
- [ ] Refactor: extraer "motor de cuotas" compartido (generación N+1, marcado paid idempotente, deuda) usado por tier y por team.
- [ ] Crear `services/mp_connect_service.go`: connect (state CSRF), callback (exchange + persist), status, webhook desautorización.
- [ ] Crear `services/platform_setting_service.go`: Get/Put de `marketplace_fee_percent` con validación de rol `admin` (rol de sistema).
- [ ] `services/payment_service.go`: rama split en CreatePreference/ProcessPayment (D9) y rama team en el webhook (D10).
- [ ] Tests unitarios (testify + mocks).

### 5. Controllers y rutas

- [ ] Crear `controllers/team_subscription_controller.go` (GET estado de cuenta).
- [ ] Crear `controllers/mp_connect_controller.go` (connect/callback/status/webhook).
- [ ] Crear `controllers/platform_setting_controller.go`.
- [ ] Registrar rutas en `cmd/api/app/url_mappings.go` (tabla D11).
- [ ] Tests de controllers.

### 6. Config e infra

- [ ] Agregar `MP_OAUTH_CLIENT_ID`, `MP_OAUTH_CLIENT_SECRET`, `MP_OAUTH_REDIRECT_URI`, `TOKEN_ENCRYPTION_KEY` a `config/config.go` y `.env.example`.
- [ ] Crear `infrastructure/crypto` (AES-GCM) con `Encrypt`/`Decrypt` para `access_token` y `refresh_token` de `seller_connections`.
- [ ] Guarda del `state` OAuth (mapa/cache en memoria con TTL).

### 7. Integración MP (Bruno)

- [ ] Testear flujo completo en sandbox: conectar entrenador (OAuth), crear equipo con `membership_fee`, agregar corredor, preferencia con split, pago, webhook → membresía activa.
- [ ] Verificar `marketplace_fee` en el detalle del pago y el `seller_user_id` en `payments`.
- [ ] Verificar doble notificación idempotente.

### 8. Swagger y documentación

- [ ] Regenerar `cmd/api/docs` con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`.
- [ ] Actualizar `README.md` (tabla de endpoints) y `.agentics/payments-integration.md` (flujo split implementado).

### 9. Verificación final

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` en verde.
- [ ] Validar escenarios de ambas specs (gate de membresía pago/gratis/aceptación, activación por webhook, doble notificación, deuda bloquea salida, estado de cuenta, OAuth connect/callback/status, comisión owner, split sin conexión).