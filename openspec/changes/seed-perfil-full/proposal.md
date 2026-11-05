# Seed de perfil full (universo demo completo)

## Por qué

El entorno local hoy tiene dos caminos: el clon de Supabase (`make local-restore`) con
datos reales, y el seed del dashboard analítico (`cmd/api/scripts/seed_dashboard/`),
que siembra solo catálogo + calendario + feedback de dos semanas pasadas y cuyos
usuarios tienen password en texto plano (no pueden loguearse).

No existe un entorno que permita usar el sistema con backend real, desde el frontend,
cubriendo todos los módulos: equipos, invitaciones, solicitudes, calendario con hoy y
futuro, entrenamientos presenciales en vivo, gamificación, competencias en todos sus
estados, pagos y cobros, mensajería. `docs/ENTORNO_LOCAL.md` lo registra como deuda
conocida ("no existe todavía").

El objetivo es un perfil `full` del seed existente: al resetear la base local, genera
un universo sintético completo y coherente, con fechas relativas a `NOW` (TZ
America/Argentina/Cordoba), personas reales logueables, y datos pasados/presentes/
futuros en todos los módulos, para usar la app sin mocks de frontend.

## Qué

Extender `cmd/api/scripts/seed_dashboard` con `--profile dashboard|full`:

- `dashboard` (default): comportamiento actual, byte-idéntico, para no afectar el
  workflow del compañero.
- `full`: universo independiente con su propio pipeline de fases y su verify
  (`verifyFull`). No ejecuta el seeder dashboard ni su verify de contadores. Team A y su
  grupo principal contienen datos protagonistas visibles en el dashboard, y el resto del
  universo extendido vive en grupos secundarios y otros equipos:
  1. **Usuarios logueables**: entrenador protagonista + 3 corredores protagonistas con
     password bcrypt `Paceron.123` (pasa `ValidatePassword`), más usuarios de borde
     (bloqueado, inactivo, sin rol, con invitaciones desactivadas).
  2. **Roles/tiers/permisos completos**: cadenas rol→tier→permisos que no rompan
     `GET /auth/permissions`, suscripciones de tier en estados activo/pendiente/deuda.
  3. **Equipos múltiples**: normal (centro de los flujos), pago, lleno, oculto,
     cerrado, y un segundo equipo del mismo entrenador para colisión presencial
     cross-team.
  4. **Catálogo ampliado**: ejercicios de todos los kinds vigentes, sesiones completas,
     planes de varias duraciones.
  5. **Calendario dinámico**: pasado con feedback/asistencias, HOY presencial abierto y
     usable al instante (más variantes no-abierto/cerrado), futuro +1..+14 con
     training/async/other/rest/cancelled.
  6. **Gamificación**: rachas vivas/rotas, logros desbloqueados y vacíos, estampas,
     unseen por sección, leaderboards con datos para las 6 métricas.
  7. **Competencias**: pending_approval, active metric, active route con intentos
     múltiples y GPS matcheable, finished con empate y ganador único, rejected, y
     límite de intentos agotado.
  8. **Pagos**: suscripciones/cuotas/pagos con historial, deuda que bloquea la salida
     del equipo, cobros del entrenador con split, seller_connection cifrada para
     `can_receive_payments=true`, y un entrenador sin conexión (409).
  9. **Mensajería y push**: mensajes de todos los tipos/modos en la sesión de hoy y en
     una pasada; push tokens de los protagonistas.
  10. **Invitaciones/solicitudes**: vigente, expirada, aceptada, rechazada; join
      request pendiente, a equipo lleno y con invitaciones desactivadas.

Además: familia nueva de Makefile `fullseed-*` (`seed`, `reset`, `cleanup`, `verify`),
verify propio de coherencia, cleanup extendido, y documentación (RUNBOOK + entorno
local).

## Alcance

- Solo tooling de desarrollo local. No toca API, DTOs, esquema de producción ni el
  perfil `dashboard`.
- El perfil full asume base local recién reseteada; usa ids de scope propios en rango
  alto y marker propio para no chocar con el perfil dashboard ni con datos reales.
- `SEEDING_ENABLED=true` + `--stage=local` siguen siendo obligatorios (guard existente).
- Para el perfil full `.env.local` necesita además `TOKEN_ENCRYPTION_KEY` (cifrado de
  seller_connections) y `MP_OAUTH_CLIENT_ID` (para `can_receive_payments`).

## No-goals

- No reemplaza `local-restore` (clon de Supabase) ni los targets `demo-*`.
- No siembra refresh tokens, estado de WebSocket ni fotos reales en storage.
- No usa MP real: todos los ids/tokens/preferencias son sintéticos.
- No cambia el comportamiento ni los contadores del perfil dashboard.
- No introduce un runner de seeds genérico: se reutiliza el script existente.

## Capabilities

Change de tooling: no modifica comportamiento observable del producto, por eso
`.openspec.yaml` declara `skip_specs: true` (mismo criterio que
`cobertura-calendario-recientes`).
