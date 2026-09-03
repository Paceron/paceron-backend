## ADDED Requirements

### Requirement: Carga de foto de perfil de usuario

El sistema SHALL permitir a un usuario autenticado cargar o reemplazar su propia foto de perfil (`PUT /api/v1/users/:id/photo`, self only). El archivo SHALL validarse por tamaño (máximo 5MB) y tipo real de contenido (magic bytes, no extensión declarada), aceptando únicamente `image/jpeg`, `image/png` o `image/webp`. Al subir exitosamente, el sistema SHALL escribir el objeto en el bucket con una key determinística (`avatars/user-{id}.{ext}`) y actualizar `photo_key`/`photo_updated_at` del usuario.

#### Scenario: Carga exitosa de foto de perfil
- **WHEN** el usuario autenticado sube una imagen válida (≤5MB, tipo permitido) a su propio `PUT /api/v1/users/:id/photo`
- **THEN** el sistema sube el archivo al bucket con la key determinística del usuario
- **AND** actualiza `photo_key` y `photo_updated_at`
- **AND** responde con la `photo_url` recalculada

#### Scenario: Reemplazo de foto existente
- **WHEN** el usuario ya tiene una foto cargada y sube una nueva
- **THEN** el sistema sobreescribe el mismo objeto en el bucket (misma key)
- **AND** actualiza `photo_updated_at`, cambiando el `?v=` de la URL devuelta

#### Scenario: Archivo demasiado grande
- **WHEN** el archivo supera 5MB
- **THEN** el sistema rechaza la carga con error `PHOTO_TOO_LARGE`, sin llegar a subir nada al bucket

#### Scenario: Tipo de archivo inválido
- **WHEN** el archivo no matchea `image/jpeg`, `image/png` ni `image/webp` según sus magic bytes, aunque el nombre o el `Content-Type` declarado digan lo contrario
- **THEN** el sistema rechaza la carga con error `PHOTO_INVALID_TYPE`

#### Scenario: Usuario intenta cargar la foto de otro usuario
- **WHEN** un usuario autenticado llama a `PUT /api/v1/users/:id/photo` con un `:id` que no es el propio
- **THEN** el sistema rechaza la operación (no autorizado)

### Requirement: Borrado de foto de perfil de usuario

El sistema SHALL permitir a un usuario autenticado borrar su propia foto de perfil (`DELETE /api/v1/users/:id/photo`, self only). El sistema SHALL eliminar el objeto del bucket y limpiar `photo_key`/`photo_updated_at` a `null`.

#### Scenario: Borrado exitoso
- **WHEN** el usuario autenticado borra su propia foto y tiene una cargada
- **THEN** el sistema elimina el objeto del bucket
- **AND** limpia `photo_key` y `photo_updated_at`

#### Scenario: Borrado sin foto previa
- **WHEN** el usuario no tiene `photo_key` seteado y llama al borrado
- **THEN** el sistema responde de forma idempotente (sin error) sin intentar borrar nada del bucket

### Requirement: Carga y borrado de ícono de equipo

El sistema SHALL permitir únicamente al entrenador dueño de un equipo cargar/reemplazar (`PUT /api/v1/teams/:id/icon`) o borrar (`DELETE /api/v1/teams/:id/icon`) el ícono del equipo. Se aplican las mismas reglas de validación de tamaño/tipo que la foto de usuario. La key determinística SHALL ser `teams/team-{id}-icon.{ext}`.

#### Scenario: Entrenador dueño carga el ícono del equipo
- **WHEN** el entrenador dueño del equipo sube una imagen válida a `PUT /api/v1/teams/:id/icon`
- **THEN** el sistema sube el archivo con la key determinística del equipo
- **AND** actualiza `icon_key`/`icon_updated_at`

#### Scenario: Usuario no dueño intenta cargar el ícono
- **WHEN** un usuario que no es el entrenador dueño del equipo (otro entrenador, un corredor miembro, o cualquier otro usuario) llama a `PUT /api/v1/teams/:id/icon`
- **THEN** el sistema rechaza la operación (no autorizado), sin subir nada al bucket

#### Scenario: Entrenador dueño borra el ícono
- **WHEN** el entrenador dueño del equipo llama a `DELETE /api/v1/teams/:id/icon` y el equipo tiene ícono cargado
- **THEN** el sistema elimina el objeto del bucket
- **AND** limpia `icon_key`/`icon_updated_at`

### Requirement: La URL pública se calcula, no se persiste

El sistema SHALL derivar `photo_url`/`icon_url` en el momento de servir la respuesta, combinando la URL base pública del bucket, la key almacenada, y un parámetro de cache-busting (`?v=`) basado en el timestamp de última actualización. El sistema SHALL NOT persistir la URL completa en ninguna tabla. Cuando `photo_key`/`icon_key` es `null`, el campo `photo_url`/`icon_url` en la respuesta SHALL ser `null`.

#### Scenario: Usuario sin foto cargada
- **WHEN** se consulta `GET /api/v1/auth/user` para un usuario con `photo_key` nulo
- **THEN** la respuesta incluye `photo_url: null`

#### Scenario: URL cambia tras un reemplazo
- **WHEN** un usuario reemplaza su foto de perfil
- **THEN** la `photo_url` devuelta en la respuesta del upload difiere de la anterior únicamente en el parámetro `?v=`, no en la key

### Requirement: Fallos de storage no corrompen el estado local

Si la subida al bucket falla, el sistema SHALL responder con error sin modificar `photo_key`/`photo_updated_at` (ni sus equivalentes de equipo), preservando el estado anterior. Si el borrado en el bucket falla, el sistema SHALL responder con error sin limpiar `photo_key`/`photo_updated_at`, para no dejar referencias a un objeto que en realidad sigue existiendo en el bucket.

#### Scenario: Falla la subida al bucket
- **WHEN** el `storageclient.Upload` devuelve error
- **THEN** el sistema responde con error 500
- **AND** no modifica `photo_key`/`photo_updated_at` del usuario o equipo

#### Scenario: Falla el borrado en el bucket
- **WHEN** el `storageclient.Delete` devuelve error
- **THEN** el sistema responde con error 500
- **AND** no limpia `photo_key`/`photo_updated_at`
