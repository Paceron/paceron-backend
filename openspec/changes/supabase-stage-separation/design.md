## Context

Confirmado con el usuario: dos proyectos de Supabase (testing/production) existen y fueron probados, pero en la práctica todo (local + el único service de Render, mal etiquetado como "producción") apuntaba al mismo — que resultó ser testing. Render además desplegaba `develop`, no `master`.

## Goals / Non-Goals

**Goals:**
- Producción solo se usa cuando alguien lo pide explícitamente — nunca por default, nunca por accidente.
- `master` en Render pasa a ser la producción real; `develop` mantiene su propio deploy contra testing (decisión del usuario: dos services, no uno).

**Non-Goals:**
- Storage S3 — priorizado por el usuario para después de esto, no se toca acá.
- Rediseñar `Environment`/`GetEnvironment()` — sigue existiendo tal cual, es un eje distinto (cómo se carga la config, no a qué Supabase pega).

## Decisions

### 1. Flag (`--stage=production`) parseado a mano, no `flag.Parse()`

**Por qué**: `config.LoadValues()` corre en un `init()` de package — se ejecuta automáticamente al importar el paquete, **antes** de que `main()` llegue a correr nada, incluido un eventual `flag.Parse()`. Usar el paquete estándar `flag` hubiera requerido mover la carga de config fuera de `init()` a una llamada explícita post-`flag.Parse()` en `main()` — un refactor más grande e innecesario para una sola bandera booleana. `os.Args` en cambio está poblado por el runtime de Go antes de que corra cualquier código, `init()` incluido — escanearlo a mano funciona sin tocar el patrón de arranque existente.

### 2. Reemplaza `DATABASE_URL`, no lo mantiene como fallback

**Por qué**: mantener `DATABASE_URL` como fallback hubiera dejado abierta la puerta de seguir usándolo "por costumbre" sin pasar por el mecanismo de stage — exactamente el problema que se está resolviendo. Corte limpio: local y Render migran a los nombres nuevos, documentado en `docs/ENVIRONMENTS.md`. El fallback histórico a `db_host`/`db_port`/etc. (sin distinción de stage) se mantiene tal cual estaba — no forma parte de este cambio, nadie confirmó si está en uso.

### 3. Log explícito del stage resuelto al arrancar

**Por qué**: sin esto, la única forma de saber a qué proyecto pegó un deploy dado sería revisar la config del service en el dashboard de Render — un paso extra y una fuente de verdad separada del comportamiento real. Loguearlo hace que sea verificable con un solo vistazo a los logs, sin ambigüedad.

### 4. Dos services de Render, no uno con branch reconfigurada

**Por qué** (decisión del usuario, ver pregunta hecha en el chat): mantener un deploy real de `develop` contra testing stage, separado del de `master`/producción, en vez de perder la posibilidad de ver develop desplegado. Trade-off aceptado: dos services en el plan free de Render (a confirmar límites de cuenta si aplica, fuera del alcance de este cambio de código).

## Risks / Trade-offs

- **Servicio nuevo de Render arranca sin las env vars no relacionadas a DB** (`JWT_SECRET`, `SMTP_*`, `GMAIL_*`) — `render.yaml` no las declaraba ni siquiera para el service existente (viven directo en el dashboard). Documentado en el checklist de `docs/ENVIRONMENTS.md`, requiere que el usuario las copie a mano al crear el service nuevo.
- **Sin backwards-compat para `DATABASE_URL`**: cualquier otro lugar no versionado en este repo que dependa de esa env var (ej. un script manual) se rompe silenciosamente hasta que se actualice. Aceptado — es exactamente el tipo de uso "por costumbre" que se quiere eliminar.
