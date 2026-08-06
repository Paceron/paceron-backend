## ADDED Requirements

### Requirement: Autenticación obligatoria por defecto
El sistema SHALL exigir un access token válido (`Authorization: Bearer <token>`) en todas las rutas, excepto las explícitamente listadas como públicas.

#### Scenario: Ruta protegida sin header
- **WHEN** se llama a cualquier ruta que no sea register/login/refresh/logout/forgot-password/reset-password/auth-user/legacy/demo/swagger, sin header `Authorization`
- **THEN** el sistema retorna HTTP 401 con `code: "unauthorized"`

#### Scenario: Ruta protegida con access token vencido
- **WHEN** se llama a una ruta protegida con un access token válido pero expirado
- **THEN** el sistema retorna HTTP 401 con `code: "token_expired"`

#### Scenario: Ruta protegida con access token válido
- **WHEN** se llama a una ruta protegida con un access token válido y vigente
- **THEN** el sistema resuelve la identidad del token y continúa el procesamiento normal del request

### Requirement: Autorización solo-entrenador
El sistema SHALL restringir la creación/actualización de equipos, grupos, y el alta de miembros/invitaciones al entrenador del equipo correspondiente.

#### Scenario: Entrenador del equipo actualiza el equipo
- **WHEN** el usuario autenticado es entrenador del equipo que intenta actualizar
- **THEN** el sistema permite la operación

#### Scenario: Usuario sin rol de entrenador intenta actualizar el equipo
- **WHEN** el usuario autenticado no es entrenador del equipo (no es miembro, o es corredor)
- **THEN** el sistema retorna HTTP 403 con `code: "Forbidden"`

### Requirement: Autorización self-o-entrenador-delegado
El sistema SHALL permitir que un usuario se remueva a sí mismo de un equipo o grupo, o que el entrenador del equipo remueva a otro miembro.

#### Scenario: Usuario se remueve a sí mismo
- **WHEN** el usuario autenticado coincide con el usuario objetivo de la remoción
- **THEN** el sistema permite la operación sin requerir rol de entrenador

#### Scenario: Entrenador remueve a otro miembro
- **WHEN** el usuario autenticado es distinto del objetivo, y es entrenador del equipo
- **THEN** el sistema permite la operación

#### Scenario: Usuario sin rol intenta remover a otro
- **WHEN** el usuario autenticado es distinto del objetivo, y no es entrenador del equipo
- **THEN** el sistema retorna HTTP 403 con `code: "Forbidden"`

### Requirement: Autorización self-only sobre datos de usuario
El sistema SHALL restringir la actualización de datos, estado y contraseña de un usuario exclusivamente al propio usuario autenticado.

#### Scenario: Usuario modifica sus propios datos
- **WHEN** el `id` del usuario en la URL coincide con el usuario autenticado
- **THEN** el sistema permite la operación

#### Scenario: Usuario intenta modificar datos de otro usuario
- **WHEN** el `id` del usuario en la URL no coincide con el usuario autenticado
- **THEN** el sistema retorna HTTP 403 con `code: "Forbidden"`, sin importar si el usuario autenticado tiene otro rol
