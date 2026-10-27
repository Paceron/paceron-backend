# Design

## Contexto

El dashboard ya resuelve scope, rango/membresía histórica, rol `main`, estado de sesión y legacy de `source_exercise_id` en `DashboardService` y `DashboardDao`. `GET /dashboard/exercises/{id}/series` expone series paginadas, pero no limita a una instancia de sesión, no entrega RPE y no es una superficie adecuada para repetir la autorización de dos lados. La nueva operación debe reutilizar el modelo y el dominio existentes, con una consulta acotada por referencia A/B.

La ruta actual de comparaciones por corredor/período es owner-only. Esta operación nueva tiene una regla propia: entrenadores comparan corredores de su scope; un miembro puede comparar solo su propio historial. La autorización se decide en backend y cada lado se valida de manera independiente.

## Decisiones

### D1: Endpoint dedicado y request por referencias completas

Agregar `POST /api/v1/dashboard/comparisons/sessions` en `url_mappings.go` y un método explícito en controller/service. El JSON se decodifica con `bindDashboardBody`, que ya rechaza campos desconocidos y ejecuta validación `binding`. Request:

```json
{
  "team_id": 29,
  "from": "2026-07-20",
  "to": "2026-10-24",
  "a": {
    "group_id": 39,
    "athlete_user_id": 24,
    "session_instance_id": 501,
    "exercise_id": 8
  },
  "b": {
    "group_id": 39,
    "athlete_user_id": 24,
    "session_instance_id": 527,
    "exercise_id": 8
  }
}
```

`exercise_id` es siempre el ID del catálogo, no `exercise_instances.id`. Cada ref identifica una combinación exacta; grupo y corredor pueden diferir entre A y B, pero ambos lados quedan bajo un único team y una ventana inclusiva. No se aceptan filtros numéricos de series: la comparación selecciona una ejecución completa; los filtros activos del dashboard deciden candidatas en el cliente, no recortan el KPI de la sesión.

### D2: Validar por completo ambos lados antes de responder

El service resuelve el scope una vez, verifica el team y valida A y B antes de formar la respuesta. Para cada lado valida que el grupo pertenezca al team y al scope, el corredor tenga autorización para ese grupo, el `session_instance_id` esté en ese grupo/fecha/rango, el feedback corresponda al atleta y al ejercicio de catálogo, y la sesión tenga serie del rol principal elegible.

- **Owner/entrenador:** ambos atletas deben estar dentro del grupo que referencia cada lado y de la membresía histórica en la fecha del feedback.
- **Miembro/corredor:** cada `athlete_user_id` debe ser igual al caller. No se compara ningún otro atleta aunque el cliente modifique el body.
- `403` para una referencia existente fuera del scope; `404` para una referencia inexistente, sin feedback o que no forma la ejecución seleccionada; `400` para estructura/rango inválido. El service no devuelve A si falla B.
- Fechas inclusivas y máximo común de dos años, mediante `parseDashboardRange`/validación existente.

La consulta de filas debe incluir en el WHERE los IDs validados del lado: team/group, fecha, atleta, `assigned_session_id`, ejercicio resuelto y `sei.role = 'main'`. Conserva el LEFT JOIN a `runner_session`, el filtro de borrado, y permite `finished`, `interrupted` o ausencia de fila. `wip` no entra.

### D3: DAO dedicado o filtro extendido, sin confiar en IDs crudos

Extender `DashboardScopeFilter`/`DashboardSeriesRow` o añadir un método dedicado de DAO para obtener todas las filas de una única ejecución. El DAO recibe IDs ya validados por el service, pero mantiene los predicados de team/grupo/fecha/atleta/sesión/ejercicio en SQL; no consulta el histórico global para filtrar en memoria.

`exercise_id` resuelve primero por `source_exercise_id`. Las instancias legacy se resuelven con el mecanismo actual `resolveLegacyInstances`: coincidencia de nombre normalizado dentro del catálogo del mismo owner. El join `session_exercise_instances` usa el par `(session_instance_id, exercise_instance_id)` porque el rol vive ahí. Las sesiones históricas sin `runner_session` se conservan con LEFT JOIN.

### D4: Agregación de dominio compartida y reglas por tipo

Reusar `dashboard.Series`, `ScopedSeries`, `Valid()`/`Achieved()` y los helpers actuales de velocidad, tiempo y validez. La suma de distancia se calcula solo sobre series válidas. La velocidad media conserva la fórmula dashboard: media de velocidad por serie ponderada por distancia. `series_count` incluye todas las filas elegibles y `valid_series` solo las utilizables según `kind`.

Agregar RPE a la fila DAO/domain desde `workout_feedback.rpe` y calcular el promedio de RPE no nulo en escala 1–10, devolviendo `rpe_count`; la ausencia de RPE no afecta validez de rendimiento.

El ranking mejor/peor se calcula por lado con el `kind` congelado:

- `ritmo_continuo`: menor/mayor `AchievedTimeMs()`; el objeto seleccionado conserva `distance_meters` real.
- `velocidad_distancia`: mayor/menor velocidad derivada (`Achieved().SpeedKph`).
- `velocidad_tiempo`: mayor/menor distancia lograda; si la distancia no existe en una fila, no es candidata al extremo aunque el tiempo pueda hacerla válida para otros KPIs.

Las filas no válidas continúan contando en `series_count`, pero no se usan para los agregados ni ranking. Los empates se resuelven por fecha, `set_number`, `feedback_id`, de forma determinista.

### D5: DTOs desacoplados de persistence y contrato utilizable en UI

Agregar request/ref y response DTO en `domains/dashboard/dto.go`. Respuesta propuesta:

```json
{
  "a": {
    "group_id": 39,
    "group_name": "Sub-18",
    "athlete_user_id": 24,
    "session_instance_id": 501,
    "session_date": "2026-08-01",
    "exercise_id": 8,
    "exercise_name": "Ritmo continuo",
    "kind": "ritmo_continuo",
    "series_count": 4,
    "valid_series": 4,
    "total_distance_m": 4600,
    "mean_speed_kph": 11.2,
    "mean_rpe": 6.5,
    "rpe_count": 4,
    "best": {
      "feedback_id": 1001,
      "set_number": 3,
      "duration_ms": 220000,
      "active_duration_ms": 215000,
      "distance_meters": 1200,
      "speed_kph": 20.1
    },
    "worst": null,
    "series": []
  },
  "b": {},
  "delta": {
    "direction": "b_minus_a",
    "speed_delta_kph": 0.8,
    "speed_delta_pct": 7.14,
    "distance_delta_m": 300
  },
  "units": {
    "duration": "ms",
    "distance": "m",
    "speed": "km/h",
    "rpe": "1-10"
  }
}
```

Los campos de cada punto A/B incluyen `feedback_id`, `set_number`, tiempos crudos, `distance_meters`, `speed_kph` y `rpe`. Los valores que no se pueden derivar serializan como JSON `null`. Las listas se ordenan por `set_number`, luego `feedback_id`.

`speed_delta_kph` es B menos A; `speed_delta_pct` es `(B − A) / A × 100`, `null` si falta una velocidad o A no es positiva. `distance_delta_m` es distancia B menos A, sobre el total de series válidas y `null` si alguno no tiene distancia calculable. En los tipos donde no se puede calcular una velocidad media, el campo queda `null`, sin sustituirlo por cero.

### D6: Arquitectura en capas y Swagger

- **Controller:** autentica con `utils.GetAuthUserID`, usa el binder estricto y el response uniforme.
- **Service:** agrega una operación nueva a `DashboardServiceInterface`, valida team/scope y ambos refs, hace lookup de nombres y delega al DAO/dominio.
- **DAO:** consulta exactamente las dos ejecuciones; no hace agregación de fórmulas.
- **Dominio:** funciones puras y tests para métricas por lado/delta; tests no requieren Gin ni DB.
- **Bootstrap/Swagger:** registrar ruta, generar documentación con request/response, y no modificar schema de base de datos.

## Riesgos / trade-offs

| Riesgo | Mitigación |
|---|---|
| Dos referencias válidas por separado podrían cruzar team o fechas no accesibles | validar cada par group/athlete/session/exercise bajo el mismo team y ventana antes de serializar cualquiera |
| La lógica de membresía actual e histórica diverge | reutilizar `resolveTeamGroup`, `validateAthletes` y el predicado SQL de ventana de membresía; tests con altas/bajas históricas |
| La media de velocidad difiere de una media aritmética simple | preservar explícitamente la media ponderada por distancia del dashboard y documentar la fórmula |
| Un exercise legacy no tiene `source_exercise_id` | reutilizar el fallback existente acotado al mismo owner y cubrirlo con integración DAO |
| RPE falta en muchos feedback antiguos | RPE nullable, `rpe_count` y `mean_rpe: null` sin cambiar la validez de la serie |
| Un request puede elegir una sesión con datos pero solo bloques secundarios | `role = 'main'` aplicado en SQL, con test de `warmup`/`cooldown` |

## Fuera de alcance

- Cambios de esquema o backfill.
- Comparación entre planes sin ejecución, nuevas comparaciones de períodos o modificación de `POST /dashboard/comparisons`.
- Cambios en frontend/mobile, caché o gráficos.
