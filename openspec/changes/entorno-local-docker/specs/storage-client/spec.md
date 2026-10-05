> Nota: esta capability no tenía spec archivada en `openspec/specs/` (solo existe `user-bank-alias` de un change anterior). Los requirements se declaran como `ADDED` completos en vez de `MODIFIED`, pero describen explícitamente el comportamiento preexistente que ya está en el código, para que la diferencia con esta rama sea legible.

## ADDED Requirements

### Requirement: Cliente S3 con path-style configurable
El sistema SHALL construir el cliente S3 con path-style por default, permitiendo desactivarlo por configuración para S3 compatibles que usen virtual-host-style.

#### Scenario: Path-style (comportamiento preexistente, default)
- **WHEN** no se pasa `ForcePathStyle` explícitamente
- **THEN** el cliente se construye con `UsePathStyle = true`, que es lo que requiere Supabase Storage

#### Scenario: Virtual-host-style
- **WHEN** se pasa `ForcePathStyle = false`
- **THEN** el cliente se construye con `UsePathStyle = false`

### Requirement: URL pública del bucket con override
El sistema SHALL construir las URLs públicas de los objetos de storage a partir de un override configurable, con un derivado como fallback para no romper el caso Supabase.

#### Scenario: Override configurado
- **WHEN** se pasa `PublicBaseURL` no vacío en las opciones del cliente
- **THEN** ese valor es la base de las URLs públicas, sin ninguna transformación

#### Scenario: Sin override, endpoint de Supabase
- **WHEN** no se pasa `PublicBaseURL` y el endpoint tiene la forma de Supabase Storage
- **THEN** la base se deriva como `https://<project-ref>.supabase.co/storage/v1/object/public/<bucket>`, que es el dominio desde el que Supabase sirve los objetos públicos (distinto del gateway S3 configurado)

#### Scenario: Sin override, endpoint de Supabase con prefijo de storage
- **WHEN** el endpoint incluye un path (por ejemplo `/storage/v1/s3`)
- **THEN** el project-ref se sigue extrayendo correctamente de la parte de host, sin que el path interfiera

### Requirement: Integridad del cliente S3
El sistema SHALL reportar como error la inicialización fallida del cliente S3, para que los servicios que lo consumen puedan distinguir "no hay storage" de un error de negocio.

#### Scenario: Configuración inválida
- **WHEN** no se pueden cargar las credenciales o la configuración base del SDK
- **THEN** `New` devuelve un error envuelto que identifica la operación fallida