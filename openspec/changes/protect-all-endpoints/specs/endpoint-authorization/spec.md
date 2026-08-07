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

### Requirement: Autorización self-only sobre asignación de roles
El sistema SHALL restringir la asignación y remoción de roles de un usuario exclusivamente al propio usuario autenticado.

#### Scenario: Usuario intenta asignar o quitar un rol de otro usuario
- **WHEN** el `id` del usuario en la URL no coincide con el usuario autenticado
- **THEN** el sistema retorna HTTP 403 con `code: "Forbidden"`

### Requirement: Activación del rol entrenador requiere verificación
El sistema SHALL exigir confirmar la contraseña actual y un alias bancario válido (propio o provisto en la solicitud) antes de activar el rol entrenador sobre el usuario autenticado.

#### Scenario: Activación exitosa con alias nuevo
- **WHEN** el usuario autenticado envía su contraseña correcta y un `bank_alias` con formato válido
- **THEN** el sistema activa el rol entrenador, persiste el alias en el perfil si vino en la solicitud, y retorna HTTP 201

#### Scenario: Activación con alias ya guardado
- **WHEN** el usuario autenticado ya tiene un `bank_alias` válido guardado y no envía uno nuevo
- **THEN** el sistema activa el rol entrenador sin requerir el campo en la solicitud

#### Scenario: Contraseña incorrecta
- **WHEN** la contraseña enviada no coincide con la del usuario autenticado
- **THEN** el sistema retorna HTTP 401 sin activar el rol

#### Scenario: Sin alias bancario disponible
- **WHEN** el usuario autenticado no tiene `bank_alias` guardado y no envía uno en la solicitud
- **THEN** el sistema retorna HTTP 400 sin activar el rol

### Requirement: Desactivación del rol entrenador bloqueada mientras lidere equipos activos
El sistema SHALL impedir que un usuario desactive su propio rol entrenador si todavía es entrenador (`RoleInTeam`) de algún equipo activo.

#### Scenario: Desactivación exitosa sin equipos activos
- **WHEN** el usuario autenticado no lidera ningún equipo activo
- **THEN** el sistema desactiva el rol entrenador

#### Scenario: Desactivación bloqueada
- **WHEN** el usuario autenticado todavía es entrenador de al menos un equipo activo
- **THEN** el sistema retorna HTTP 409 sin desactivar el rol
