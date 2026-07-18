## ADDED Requirements

### Requirement: El campo bank_alias SHALL ser persistido en la base de datos

El sistema SHALL almacenar el alias bancario del usuario como una columna nullable en la tabla `users`. El campo SHALL aceptar valores `NULL` para usuarios que no configuren un alias bancario.

#### Scenario: Usuario actualiza bank_alias con valor válido
- **WHEN** se envía una solicitud PUT a `/api/v1/users/:id` con un campo `bank_alias` que cumple las reglas de validación
- **THEN** el sistema persiste el valor en la columna `bank_alias` de la tabla `users`

#### Scenario: Usuario envía bank_alias nulo
- **WHEN** se envía una solicitud PUT a `/api/v1/users/:id` sin incluir el campo `bank_alias` o con valor `null`
- **THEN** el sistema mantiene el valor existente de `bank_alias` en la base de datos sin modificarlo

### Requirement: El sistema SHALL validar el formato de bank_alias

El sistema SHALL rechazar solicitudes donde el campo `bank_alias` no cumpla con las siguientes reglas:
- Longitud mínima: 6 caracteres.
- Longitud máxima: 20 caracteres.
- Caracteres permitidos: letras minúsculas (a-z), letras mayúsculas (A-Z), números (0-9), puntos (.) y guiones (-).

#### Scenario: bank_alias con longitud menor a 6 caracteres
- **WHEN** se envía un `bank_alias` con menos de 6 caracteres
- **THEN** el sistema responde con un error HTTP 400 con mensaje indicando que la longitud debe ser entre 6 y 20 caracteres

#### Scenario: bank_alias con longitud mayor a 20 caracteres
- **WHEN** se envía un `bank_alias` con más de 20 caracteres
- **THEN** el sistema responde con un error HTTP 400 con mensaje indicando que la longitud debe ser entre 6 y 20 caracteres

#### Scenario: bank_alias con caracteres no permitidos
- **WHEN** se envía un `bank_alias` que contiene caracteres distintos a letras, números, puntos o guiones
- **THEN** el sistema responde con un error HTTP 400 con mensaje indicando que el formato no es válido

#### Scenario: bank_alias con formato válido
- **WHEN** se envía un `bank_alias` de entre 6 y 20 caracteres que contiene solo letras, números, puntos o guiones
- **THEN** el sistema acepta el campo y lo persiste en la base de datos

### Requirement: El sistema SHALL incluir bank_alias en la respuesta de actualización

El sistema SHALL retornar el campo `bank_alias` en la respuesta JSON del endpoint PUT `/api/v1/users/:id`.

#### Scenario: Respuesta exitosa incluye bank_alias
- **WHEN** la actualización del usuario se completa exitosamente
- **THEN** la respuesta JSON incluye el campo `bank_alias` con el valor persistido

#### Scenario: Respuesta con bank_alias nulo
- **WHEN** la actualización se completa y el usuario no tiene `bank_alias` configurado
- **THEN** la respuesta JSON incluye el campo `bank_alias` con valor `null` o lo omite según el comportamiento de `omitempty`

### Requirement: El sistema SHALL aplicar TrimSpace al bank_alias

El sistema SHALL eliminar espacios en blanco al inicio y final del valor de `bank_alias` antes de persistirlo en la base de datos.

#### Scenario: bank_alias con espacios en blanco
- **WHEN** se envía un `bank_alias` con espacios en blanco al inicio o final (ej: `"  mi-alias  "`)
- **THEN** el sistema persiste el valor sin los espacios exteriores (ej: `"mi-alias"`)
