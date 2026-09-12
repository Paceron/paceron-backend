## Context

`teams.membership_fee` (numeric, default 0, columna existente) es el dato que dispara todo el flujo de suscripción al equipo: el gate `ApplyTeamMembershipGate` lo usa al dar de alta corredores (`membership_fee == 0` → membresía directa `active`; `> 0` → `first_payment_pending` + cuota #1 con split). El valor se congela en `team_users.init_amount` al momento de la unión.

Pero los endpoints CRUD de equipos no lo exponen:
- `CreateTeamRequest` (`cmd/api/domains/team/team_request.go`) no lo acepta → todo equipo nuevo nace con `0`.
- `UpdateTeamRequest` (`team_update_request.go`) no lo acepta.
- `TeamResponse` (`team_response.go`) no lo devuelve, y `toResponse()` (`team_service.go:509`) no lo mapea.

Resultado: setear la mensualidad exige un `UPDATE` por SQL en la base (workaround documentado en `docs/CU/02-pago-participacion-equipo.md` y `docs/CAMBIO_SUSCRIPCION_TEAMS_SPLIT_TESTING.md`).

## Goals / Non-Goals

**Goals:**
- `POST /api/v1/teams` acepta `membership_fee` (opcional, `0` = gratis) y lo persiste.
- `PUT /api/v1/teams/:id` lo actualiza de forma parcial (como el resto de los campos).
- `GET /api/v1/teams/:id` y `GET /api/v1/teams` lo incluyen en la respuesta.
- Validar `membership_fee >= 0` en ambos requests; negativo → `400`.

**Non-Goals:**
- **No** cambiar membresías existentes al modificar el fee (`init_amount` ya está congelado).
- **No** exponer `membership_fee` en `GET /teams/search` (`TeamSearchResult`): queda para un cambio futuro si el frontend lo pide en las tarjetas de búsqueda.
- **No** tocar el gate de membresía, los endpoints de alta/invitación, ni el flujo de pagos/split.
- **No** cambios de schema (la columna ya existe).

## Decisions

### D1. Requests con `*float64` y defaults

- `CreateTeamRequest.MembershipFee *float64` — `nil` → `0` (gratis). Es el mismo patrón de consistencia que el resto de campos opcionales del update (punteros).
- `UpdateTeamRequest.MembershipFee *float64` — `nil` → no se toca el campo (actualización parcial).
- `TeamResponse.MembershipFee float64` — siempre presente, `0` si gratis.

### D2. Validación `>= 0` con sentinel y código de error propio

- En el service: `ErrInvalidMembershipFee` (sentinel en `team_service.go`, mismo patrón que `ErrInvalidQuery`/`ErrPhotoTooLarge`), mensaje `"membership_fee no puede ser negativo"`.
- En `team_controller.go`, tanto `Create` como `Update` mapean `errors.Is(err, services.ErrInvalidMembershipFee)` → `400` con `code = "INVALID_MEMBERSHIP_FEE"`.
- Se valida **antes** de persistir (en `Create` antes del `Create` del DAO; en `Update` dentro del bloque de asignación de campo).
- Razonamiento de dónde validar: la validación vive en el service (no en el controller) porque es regla de negocio, testeable contra el service directamente y consistente con los sentinels existentes.

### D3. Sin retroactividad en el update

Cambiar `membership_fee` solo afecta **altas futuras** (el gate lee el valor actual al crear el `team_user`). Las membresías existentes conservan su `init_amount`. Casos:
- `fee 0 → >0`: las altas futuras quedan con primer pago; las membresías previas (gratis, sin `subscription_status`) no cambian.
- `fee >0 → 0`: las altas futuras quedan gratis; las membresías ya `first_payment_pending` siguen su ciclo de pago hasta resolverse (no se cancelan solas).

Esto queda explícito en el spec y en la doc (`CAMBIO_SUSCRIPCION_TEAMS_SPLIT_TESTING.md`) para que el tester no lo confunda con un cobro retroactivo.

### D4. Un solo DTO de respuesta

`TeamResponse` es compartido por `Create`, `Update`, `GetByID` y `GetAll`. Agregar `MembershipFee` en `toResponse()` cubre los 4 endpoints de lectura/escritura a la vez; no hace falta tocar `GetAll` ni `GetByID`.

### D5. Swagger y docs

- Regenerar swagger (el campo aparece en los schemas de request/response automáticamente al agregarlo a los structs).
- Actualizar godoc de `Create`/`Update` solo si hace falta aclarar el campo (los `@Param body` ya referencian los structs, no cambia la firma).
- `README.md` (tabla de endpoints) y los dos docs de testing de equipos dejan de instruir el workaround SQL y pasan a decir "configurable por API".

## Risks / Trade-offs

- **Sin retroactividad puede sorprender al frontend** si asume que bajar el fee recalcula lo que ya deben los corredores: se documenta explícitamente en la respuesta del spec y en la doc de testing. Trade-off aceptado: es el comportamiento natural del diseño actual (`init_amount` congelado) y cambiarlo sería un cambio de producto, no de API.
- **Comparación de floats**: `membership_fee >= 0` con `float64` es suficiente para esta validación; no hay aritmética, solo guard y persistencia del valor tal cual viene. El gate ya compara `membershipFee == 0` de la misma forma hoy.
- **No exponer el fee en search** deja un endpoint de lectura (search) sin el campo mientras `GetByID`/`GetAll` sí lo tienen: incoherencia menor y acotada, a propósito (ver Non-Goals).