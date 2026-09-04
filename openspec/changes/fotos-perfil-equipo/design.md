# Design: Fotos de perfil de usuario e ícono de equipo

## Context

`users` y `teams` no tienen columna de imagen. El storage S3-compatible de Supabase (dos proyectos, testing/producción, mismo split que la DB — ver `docs/ENVIRONMENTS.md`) tiene credenciales ya provistas por el usuario y probadas contra un backend standalone, pero no hay código en `paceron-backend` que lo use — quedó explícitamente deprioritizado hasta tener una feature concreta que lo consuma (ver memoria de sesión). Foto de perfil y foto/ícono de equipo cierran ese hueco.

Certificados/diplomas/certificaciones (documentos, no fotos) se identificaron como necesidad futura pero quedan fuera de este change — ver Non-Goals en `proposal.md`.

## Goal

- Permitir cargar, reemplazar y borrar la foto de perfil de un usuario (self only) y el ícono de un equipo (solo el entrenador dueño).
- Servir esas imágenes desde un bucket público de Supabase Storage, sin exponer credenciales al frontend.
- Dejar la integración de storage (`restclients/storageclient`) genérica, para que soportar documentos más adelante no requiera rediseñarla.

## Non-Goals

Ver `proposal.md` — documentos, URL presignada, edición de imagen, historial de versiones.

## Decisions

### D1. Transporte del upload: proxy por el backend, no URL presignada

Dos opciones evaluadas:

- **Proxy (elegida)**: el frontend manda el archivo por `multipart/form-data` al backend, que lo valida y lo sube a Supabase. Un solo llamado, sin configurar CORS en el bucket.
- **Presignada**: el backend genera una URL PUT firmada, el frontend sube directo a Supabase. Menos carga en Render, pero requiere CORS en el bucket y dos llamados coordinados.

Para el tamaño real del caso (fotos de perfil/ícono, tope 5MB, tráfico bajo de un MVP), el ahorro de banda de la opción presignada no compensa el setup extra de CORS. El archivo nunca se persiste en disco de Render — entra por el request, se reenvía a Supabase en el mismo handler, no queda guardado — así que el riesgo de saturar el free tier es bajo con los límites de tamaño de D4.

**Migración futura**: si el caso de uso crece (archivos grandes, mucho volumen), migrar a presignada es un cambio acotado al `storageclient` + un endpoint que genere la URL firmada — no afecta el modelo de datos (D2) ni el resto de la arquitectura.

### D2. Modelo de datos: key + timestamp, no URL completa

`users`:
| columna | tipo | notas |
|---|---|---|
| `photo_key` | string, nullable | key del objeto en el bucket, ej. `avatars/user-42.jpg` |
| `photo_updated_at` | timestamp, nullable | fecha del último upload, usada para cache-busting |

`teams`:
| columna | tipo | notas |
|---|---|---|
| `icon_key` | string, nullable | ej. `teams/team-7-icon.png` |
| `icon_updated_at` | timestamp, nullable | |

La URL pública **no se guarda** — se calcula al serializar la respuesta: `{public_base_url}/{key}?v={unix(updated_at)}`, donde `public_base_url` se **deriva** de `MyStorage.Endpoint` + `MyStorage.Bucket` (`storageclient.PublicBaseURL`, parsea el project-ref del endpoint S3 y arma `https://<project-ref>.supabase.co/storage/v1/object/public/<bucket>`) — no hay env var `_PUBLIC_BASE_URL` (desvío del diseño original: se eliminó tras confirmar en la prueba manual que el project-ref ya está en el endpoint, una env var aparte era redundante). `photo_key`/`icon_key` nulos → `photo_url`/`icon_url` nulos en la respuesta (sin foto cargada).

### D3. Key fija por entidad, resubir pisa el archivo

Key determinística: `avatars/user-{id}.{ext}` / `teams/team-{id}-icon.{ext}`, con `{ext}` derivado del content-type real validado (D4), no de lo que mande el cliente en el nombre del archivo. Resubir una foto sobreescribe el mismo objeto — no hay que rastrear ni limpiar archivos anteriores, y un reintento de upload (ej. por timeout de red) es idempotente porque pisa lo mismo.

Trade-off asumido: la URL no cambia entre uploads, por eso el cache-busting de D2 es necesario (si no, el navegador podría mostrar la foto vieja cacheada tras un reemplazo).

**Caso borde — cambio de extensión**: si un usuario sube un `.png` y después reemplaza con un `.jpg`, la key cambia (`avatars/user-42.png` → `avatars/user-42.jpg`) y queda un objeto huérfano con la extensión vieja en el bucket. Aceptado como trade-off menor (un archivo de pocos KB, no crece indefinidamente porque cada usuario tiene como mucho 2 extensiones posibles a la vez) — no se implementa borrado del objeto con la key anterior en el upload de reemplazo, para no acoplar el flujo de upload a una lectura previa del `photo_key` actual. Si se vuelve un problema real (auditoría de bucket, costos), se resuelve leyendo la key anterior antes de subir la nueva y borrándola después.

### D4. Validación de archivo

- Tamaño máximo: 5MB — rechazo temprano si `Content-Length` lo supera, antes de leer el body completo.
- Tipos permitidos: `image/jpeg`, `image/png`, `image/webp`.
- Validación por **magic bytes** (primeros bytes del archivo), no por extensión del nombre ni por el `Content-Type` que declare el cliente — evita que se suba un archivo disfrazado de imagen.
- Códigos de error de dominio: `PHOTO_TOO_LARGE`, `PHOTO_INVALID_TYPE` (usar `domains/apierror` existente).

### D5. Componentes y capas

Sigue `Controllers → Services → DAOs/RestClients → Infrastructure` (`.agentics/CONVENTIONS.md`):

- **`restclients/storageclient`** (nuevo, mismo patrón que `restclients/mercadopagoclient` — es un cliente de API externa, no infra transversal): interfaz `StorageClientInterface` con `Upload(ctx, key string, content []byte, contentType string) error` y `Delete(ctx, key string) error`, sobre `aws-sdk-go-v2/service/s3` (path-style, endpoint custom). `[]byte` en vez de `io.Reader`: el service ya lee el archivo completo para validar magic bytes (D4), acotado a 5MB — sidestep de cualquier caso borde de streams no-seekable con firmado SigV4. Sin lógica de negocio, sin conocer `user`/`team`.
- **Config**: `StorageConfig`/`MyStorage` en `config/config.go`, cargado igual que `MyMP`/`MyMailer` — resuelve testing/producción vía `config.IsProductionStage()` (reusa el flag existente, no inventa uno nuevo).
- **DAOs**: `daos/user_dao.go` gana `UpdatePhoto(ctx, userID, key, updatedAt)` / `ClearPhoto(ctx, userID)`; `daos/team_dao.go` gana `UpdateIcon(ctx, teamID, key, updatedAt)` / `ClearIcon(ctx, teamID)`.
- **Services**: se extienden `services/user_service.go` (`UploadPhoto`, `DeletePhoto`) y `services/team_service.go` (`UploadIcon`, `DeleteIcon` — valida que el caller sea el entrenador dueño del equipo, mismo patrón de autorización que ya usan otras operaciones de equipo). No se crean servicios nuevos: el alcance (2 entidades, 4 operaciones) no justifica un `PhotoService` separado — si documentos (Non-Goals) se agrega después y el caso crece, ahí se evalúa extraer.
- **Controllers**: se extienden `controllers/user_controller.go` y `controllers/team_controller.go`.

### D6. Endpoints

| Método | Path | Quién | Body | Respuesta |
|---|---|---|---|---|
| PUT | `/api/v1/users/:id/photo` | self only | multipart, campo `photo` | `{ photo_url }` |
| DELETE | `/api/v1/users/:id/photo` | self only | — | 204 |
| PUT | `/api/v1/teams/:id/icon` | entrenador dueño del equipo | multipart, campo `photo` | `{ icon_url }` |
| DELETE | `/api/v1/teams/:id/icon` | entrenador dueño del equipo | — | 204 |

No hay `GET` dedicado: `photo_url` se agrega a la respuesta de `GET /api/v1/auth/user` (y donde ya se devuelva el usuario), `icon_url` a `GET /api/v1/teams/:id`.

### D7. Flujo de upload

1. Controller recibe multipart, chequea `Content-Length` contra el máximo (D4) antes de leer el body completo.
2. Lee el archivo, valida magic bytes → content-type real; si no matchea uno permitido, 400 con `PHOTO_INVALID_TYPE`.
3. Service arma la key determinística (D3) según el content-type validado.
4. `storageclient.Upload(ctx, key, reader, contentType)` — PUT al bucket de Supabase.
5. Si el upload a Supabase fallа: 500, no se toca la DB (queda la foto anterior intacta, el estado previo es válido).
6. Si el upload fue exitoso: DAO actualiza `photo_key`/`photo_updated_at` (o `icon_key`/`icon_updated_at`). Si esto falla, se loguea el error (mismo patrón best-effort que mail/push en este repo) — queda una inconsistencia temporal entre bucket y DB, aceptable para este alcance (no hay transacción distribuida cross-sistema).
7. Respuesta: URL calculada con timestamp fresco (D2).

### D8. Flujo de borrado

1. DAO busca `photo_key`/`icon_key` actual — si es nulo, 404 (nada que borrar) o 204 (idempotente); se documenta como 204 idempotente para no forzar al frontend a chequear estado antes de llamar.
2. `storageclient.Delete(ctx, key)`.
3. Si el borrado en Supabase falla: 500, no se limpia la DB (evita quedar con `photo_key` apuntando a un objeto que en realidad sigue existiendo, sería peor que el inverso).
4. Si el borrado fue exitoso: DAO limpia `photo_key`/`photo_updated_at` a `null`.

### D9. Bucket público

El bucket de storage se configura como lectura pública — las fotos de perfil/ícono de equipo no son información sensible, y esto evita que cada `<img>` del frontend tenga que pasar por el backend o pedir una URL firmada de lectura. Las credenciales de escritura (access key/secret) solo viven en el backend (env vars de Render), nunca se exponen al frontend.

## Risks / Trade-offs

- **Objetos huérfanos por cambio de extensión** (D3): aceptado, impacto mínimo (archivos chicos, acotados por usuario).
- **Inconsistencia bucket/DB en fallos parciales** (D7/D8): mismo patrón best-effort ya usado en el repo para mail/push — no bloquea la operación principal por un fallo secundario, se loguea para investigar manualmente si aparece.
- **Sin URL presignada todavía**: si el tráfico o tamaño de archivo crece mucho, revisar D1 — el cambio queda acotado al `storageclient` y no afecta el modelo de datos.

## Follow-up

- Documentos (certificados médicos, diplomas, certificaciones): nuevo change cuando haya una feature concreta que los consuma — reusa `restclients/storageclient` tal cual, agrega columnas/endpoints propios (probablemente con historial, a diferencia de fotos que solo guardan la vigente).
- Si se necesita servir imágenes redimensionadas (thumbnails), evaluar Supabase Image Transformation (ya disponible en el storage S3-compatible) antes de procesar imágenes en el backend.
