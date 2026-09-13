## ADDED Requirements

### Requirement: Crear equipo con `membership_fee`

El sistema SHALL aceptar `membership_fee` (numeric, opcional) en `POST /api/v1/teams`. Si no se envía, SHALL usar `0` (equipo gratis). SHALL rechazar con `400` y código `INVALID_MEMBERSHIP_FEE` un valor menor a `0`.

#### Escenario: crear equipo pago
- **WHEN** un entrenador crea un equipo con `membership_fee = 5000`
- **THEN** el equipo se persiste con `membership_fee = 5000` y la respuesta de creación lo incluye

#### Escenario: crear equipo gratis (default)
- **WHEN** un entrenador crea un equipo sin enviar `membership_fee`
- **THEN** el equipo se persiste con `membership_fee = 0` y la respuesta lo incluye como `0`

#### Escenario: membership_fee negativo
- **WHEN** un entrenador crea un equipo con `membership_fee = -1`
- **THEN** la creación se rechaza con `400` y código `INVALID_MEMBERSHIP_FEE`, sin persistir nada

### Requirement: Actualizar `membership_fee`

El sistema SHALL aceptar `membership_fee` (opcional, actualización parcial) en `PUT /api/v1/teams/:id`, con la misma validación `>= 0`. El cambio SHALL NO alterar membresías existentes (el `init_amount` de cada `team_user` queda congelado al valor de su unión); solo afecta futuras altas de corredores.

#### Escenario: actualizar la mensualidad
- **WHEN** el entrenador actualiza `membership_fee = 8000` en su equipo
- **THEN** el equipo queda con `membership_fee = 8000`, la respuesta lo incluye y las membresías existentes no cambian

#### Escenario: no envía membership_fee
- **WHEN** el entrenador actualiza otros campos sin enviar `membership_fee`
- **THEN** el `membership_fee` del equipo permanece sin cambios

#### Escenario: membership_fee negativo
- **WHEN** el entrenador actualiza con `membership_fee = -5`
- **THEN** la actualización se rechaza con `400` y código `INVALID_MEMBERSHIP_FEE`, sin modificar el equipo

### Requirement: Leer `membership_fee` del equipo

El sistema SHALL incluir `membership_fee` en la respuesta de `GET /api/v1/teams/:id` (y, por usar el mismo DTO, en `GET /api/v1/teams` y en las respuestas de `POST`/`PUT`).

#### Escenario: leer un equipo pago
- **WHEN** se consulta un equipo cuyo `membership_fee = 8000`
- **THEN** la respuesta incluye `membership_fee = 8000`

#### Escenario: leer un equipo gratis
- **WHEN** se consulta un equipo cuyo `membership_fee = 0`
- **THEN** la respuesta incluye `membership_fee = 0`