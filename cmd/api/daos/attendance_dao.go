package daos

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgconn"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// ErrAttendanceAlreadyExists indica que la asistencia ya fue registrada: la base
// rechazó el insert por la constraint UNIQUE (team_id, training_session_id, user_id).
var ErrAttendanceAlreadyExists = errors.New("esta asistencia fue previamente registrada")

// postgresUniqueViolation es el SQLSTATE que Postgres reporta al violar una UNIQUE o PK.
// attendanceUniqueConstraint es el nombre del índice único compuesto
// (team_id, training_session_id, user_id) declarado en dbs.Attendance. Lo compara
// el Create para NO confundir una violación suya con la de otro índice.
const attendanceUniqueConstraint = "uq_att_team_session_user"

const postgresUniqueViolation = "23505"

// AttendanceSessionContext es el contexto completo de la sesión objetivo de una
// operación de asistencia: la sesión instanciada más el día de calendario que la
// respalda, con su grupo y su equipo. Se resuelve en una sola query para
// que las 5 validaciones de resolveAttendanceSession no puedan discrepar entre
// sí ni con la grilla (design.md D1/D11).
type AttendanceSessionContext struct {
	SessionInstanceID  int64
	SessionName        string
	Date               time.Time
	Kind               string
	IsPresencial       bool
	PresencialTimeFrom *time.Time
	PresencialTimeTo   *time.Time
	PresencialLocation *string
	GroupID            int64
	GroupName          string
	TeamID             int64
	TeamName           string
}

// AttendanceSearchFilters agrupa los filtros opcionales de búsqueda de asistencias.
// Los valores llegan ya autorizados por el service (team_id obligatorio + rol en el
// team): el DAO solo construye el WHERE.
type AttendanceSearchFilters struct {
	TeamID            *int64
	TrainingSessionID *int64
	UserID            *int64
}

// SessionAttendanceRow es una fila de la grilla de asistencia: un corredor del
// grupo cruzado con el estado de su asistencia para la sesión. La compone el
// service (no es una tabla): UserID y AttendanceID vienen del JOIN, Name/Email
// del batch lookup de usuarios y Status se deriva de AttendanceID.
type SessionAttendanceRow struct {
	UserID       int64
	Name         string
	Email        string
	AttendanceID *int64
	Status       string
	Source       *string
	RegisteredAt *time.Time
}

// SessionAttendanceOption es una sesión presencial ya ocurrida del grupo, con el
// conteo de asistencias que tiene cargadas.
type SessionAttendanceOption struct {
	SessionInstanceID  int64      `gorm:"column:session_instance_id"`
	Name               string     `gorm:"column:name"`
	Date               time.Time  `gorm:"column:date"`
	PresencialTimeFrom *time.Time `gorm:"column:presencial_time_from"`
	PresencialTimeTo   *time.Time `gorm:"column:presencial_time_to"`
	AttendedCount      int64      `gorm:"column:attended_count"`
}

// AttendanceDAOInterface define las operaciones de acceso a datos para asistencias.
type AttendanceDAOInterface interface {
	Create(ctx *gin.Context, attendance *dbs.Attendance) error
	Search(ctx *gin.Context, filters AttendanceSearchFilters) ([]dbs.Attendance, error)
	TeamExists(ctx *gin.Context, teamID int64) (bool, error)
	IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error)
	ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error)
	GetTeamUserRole(ctx *gin.Context, teamID, userID int64) (string, error)
	FindSessionContext(ctx *gin.Context, sessionInstanceID int64) (*AttendanceSessionContext, error)
	FindPastPresencialSessionsForGroup(ctx *gin.Context, groupID, teamID int64) ([]SessionAttendanceOption, error)
	FindGroupRosterWithAttendance(ctx *gin.Context, groupID, teamID, sessionInstanceID int64, sessionDate time.Time) ([]SessionAttendanceRow, *SessionAttendanceAggregates, error)
	BulkUpsertManual(ctx *gin.Context, teamID, sessionInstanceID, actorUserID int64, userIDs []int64) (created int, updated int, err error)
	FindByID(ctx *gin.Context, attendanceID int64) (*dbs.Attendance, error)
	DeleteByID(ctx *gin.Context, attendanceID, teamID int64) (bool, error)
}

type attendanceDao struct {
	DB         *gorm.DB
	membership TeamMembershipDAOInterface
}

// NewAttendanceDao crea una nueva instancia de AttendanceDao. Los chequeos de
// ownership/pertenencia de equipos (TeamExists, IsTeamOwner,
// ExistsUserInTeamOwnedBy) delegan en TeamMembershipDAO, compartido con otros
// módulos — ver daos/team_membership_dao.go.
func NewAttendanceDao(database *gorm.DB) AttendanceDAOInterface {
	return &attendanceDao{
		DB:         database,
		membership: NewTeamMembershipDao(database),
	}
}

// Create inserta una asistencia. Si la base rechaza el insert por la constraint
// UNIQUE (asistencia duplicada del mismo (team_id, training_session_id, user_id)),
// devuelve ErrAttendanceAlreadyExists. No se hace un SELECT previo a propósito:
// insert directo + captura de la violación evita race conditions.
func (d *attendanceDao) Create(ctx *gin.Context, attendance *dbs.Attendance) error {
	err := d.DB.Create(attendance).Error
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == postgresUniqueViolation {
		// Solo el índice compuesto significa "ya estaba". La tabla tiene otro
		// unique —el pkey de `id`— y mapear cualquier violación a
		// ErrAttendanceAlreadyExists hacía que un problema distinto se le dijera al
		// corredor "ya tenías la asistencia registrada" cuando en realidad no se
		// registró nada. Se chequea el constraint: la pkey no se puede violar con un
		// insert normal (va por secuencia), pero el chequeo explícito evita que
		// este día se convierta en un "ya registrada" fantasma si algún día aparece
		// otra restricción.
		if pgErr.ConstraintName == attendanceUniqueConstraint {
			return ErrAttendanceAlreadyExists
		}
		return fmt.Errorf("error creating attendance: unique violation on %q (no es el índice de duplicado): %w", pgErr.ConstraintName, err)
	}
	return fmt.Errorf("error creating attendance: %w", err)
}

// Search devuelve las asistencias que cumplen los filtros dados (WHERE dinámico).
// Los filtros llegaron ya autorizados por el service.
func (d *attendanceDao) Search(ctx *gin.Context, filters AttendanceSearchFilters) ([]dbs.Attendance, error) {
	query := d.DB.Model(&dbs.Attendance{})
	if filters.TeamID != nil {
		query = query.Where("team_id = ?", *filters.TeamID)
	}
	if filters.TrainingSessionID != nil {
		query = query.Where("training_session_id = ?", *filters.TrainingSessionID)
	}
	if filters.UserID != nil {
		query = query.Where("user_id = ?", *filters.UserID)
	}

	var attendances []dbs.Attendance
	if err := query.Order("id").Find(&attendances).Error; err != nil {
		return nil, fmt.Errorf("error searching attendances: %w", err)
	}
	return attendances, nil
}

// TeamExists indica si existe un team activo (sin soft-delete) con ese id.
func (d *attendanceDao) TeamExists(ctx *gin.Context, teamID int64) (bool, error) {
	return d.membership.TeamExists(ctx, teamID)
}

// IsTeamOwner indica si userID es el owner del team (teams.owner_id).
func (d *attendanceDao) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	return d.membership.IsTeamOwner(ctx, teamID, userID)
}

// ExistsUserInTeamOwnedBy indica si targetUserID pertenece (team_users activo) a
// al menos un team cuyo owner sea ownerUserID. Es la relación que autoriza el
// Escenario C de la matriz de búsqueda: un owner puede ver asistencias de un
// corredor solo si ese corredor está en un equipo que él administra.
func (d *attendanceDao) ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
	return d.membership.ExistsUserInTeamOwnedBy(ctx, targetUserID, ownerUserID)
}

// GetTeamUserRole devuelve el rol (entrenador/corredor) de userID en teamID si
// pertenece activamente ("" si no es miembro).
func (d *attendanceDao) GetTeamUserRole(ctx *gin.Context, teamID, userID int64) (string, error) {
	return d.membership.GetTeamUserRole(ctx, teamID, userID)
}

// findSessionContextSQL resuelve en una sola query el contexto de la sesión
// objetivo. El JOIN a groups y teams existe solo para traer group_id/team_id (que
// el service necesita para autorizar) y los nombres que viajan en el bloque
// "session" de la respuesta; no es una decisión de seguridad en sí — la
// autorización real la hace el service contra el team_id resuelto acá.
const findSessionContextSQL = `
SELECT
	si.id                    AS session_instance_id,
	si.name                  AS session_name,
	gcd.date                 AS date,
	gcd.kind                 AS kind,
	gcd.is_presencial        AS is_presencial,
	gcd.presencial_time_from AS presencial_time_from,
	gcd.presencial_time_to   AS presencial_time_to,
	gcd.presencial_location  AS presencial_location,
	g.id                     AS group_id,
	g.name                   AS group_name,
	g.team_id                AS team_id,
	t.name                   AS team_name
FROM session_instances si
JOIN group_calendar_days gcd ON gcd.session_instance_id = si.id
JOIN groups g              ON g.id = gcd.group_id
JOIN teams t               ON t.id = g.team_id
WHERE si.id = ?
ORDER BY gcd.id
LIMIT 1`

// FindSessionContext devuelve el contexto de la sesión objetivo (instancia + día
// de calendario + grupo + equipo), o (nil, nil) si la sesión no existe o no
// está asignada a ningún día de calendario. El service traduce ese nil a 404:
// son indistinguibles a propósito, para no filtrar la existencia de una sesión
// huérfana.
//
// El ORDER BY gcd.id + LIMIT 1 es defensivo: el modelo dice 1:1 entre
// session_instances y group_calendar_days, pero si algún día una instancia
// quedara referenciada por dos días el resultado debe ser estable entre llamadas
// en lugar de depender del plan de ejecución.
func (d *attendanceDao) FindSessionContext(ctx *gin.Context, sessionInstanceID int64) (*AttendanceSessionContext, error) {
	var result AttendanceSessionContext
	row := d.DB.Raw(findSessionContextSQL, sessionInstanceID).Scan(&result)
	if row.Error != nil {
		return nil, fmt.Errorf("error finding session context: %w", row.Error)
	}
	if row.RowsAffected == 0 {
		return nil, nil
	}
	return &result, nil
}

// findPastPresencialSessionsSQL arma el listado de sesiones sobre las que se
// puede gestionar asistencia. Los tres filtros de la spec (kind = 'training',
// is_presencial, date <= hoy) van en el WHERE, no como post-filtro en Go.
//
// `date <= CURRENT_DATE` se deja en SQL a proposito (SARGable, usa el indice
// idx_group_calendar_day_group_date) y NO se convierte a time.Time para
// comparar en Go: convertirlo obligaria a traer todos los dias del grupo para
// descartar los futuros en memoria (D11). El criterio es el del servidor de
// Postgres, igual que el resto del modulo de calendario (R2).
//
// El equipo se filtra por DOS lados a proposito: g.team_id verifica que el grupo
// sea del equipo de la operacion, y a.team_id evita contar asistencias de otro
// equipo que compartieran session_instance_id por un dato inconsistente.
const findPastPresencialSessionsSQL = `
SELECT
	si.id             AS session_instance_id,
	si.name           AS name,
	gcd.date          AS date,
	gcd.presencial_time_from AS presencial_time_from,
	gcd.presencial_time_to   AS presencial_time_to,
	COUNT(a.id)       AS attended_count
FROM group_calendar_days gcd
JOIN groups g             ON g.id = gcd.group_id AND g.team_id = ?
JOIN session_instances si  ON si.id = gcd.session_instance_id
LEFT JOIN attendances a    ON a.training_session_id = si.id AND a.team_id = ?
WHERE gcd.group_id = ?
  AND gcd.kind = 'training'
  AND gcd.is_presencial = true
  AND gcd.date <= CURRENT_DATE
  AND gcd.session_instance_id IS NOT NULL
GROUP BY si.id, si.name, gcd.date, gcd.presencial_time_from, gcd.presencial_time_to
ORDER BY gcd.date DESC`

// FindPastPresencialSessionsForGroup devuelve las sesiones presenciales ya
// ocurridas del grupo, ordenadas de la mas reciente a la mas antigua, con su
// conteo de asistencias.
//
// teamID aparece dos veces en la query (grupo y asistencias) y se pasa en el
// mismo orden que los placeholders del SQL: grupo primero, luego asistencias.
func (d *attendanceDao) FindPastPresencialSessionsForGroup(ctx *gin.Context, groupID, teamID int64) ([]SessionAttendanceOption, error) {
	var result []SessionAttendanceOption
	row := d.DB.Raw(findPastPresencialSessionsSQL, teamID, teamID, groupID).Scan(&result)
	if row.Error != nil {
		return nil, fmt.Errorf("error listing past presencial sessions: %w", row.Error)
	}
	return result, nil
}

// SessionAttendanceAggregates son los dos conteos crudos de la grilla, tal como
// salen de la base. El service deriva de ellos los agregados de la respuesta
// (not_confirmed y el porcentaje).
type SessionAttendanceAggregates struct {
	// RosterSize es la cantidad de miembros del grupo con membresia activa en la
	// fecha de la sesion.
	RosterSize int64
	// Attended es la cantidad TOTAL de asistencias de la sesion, sin filtrar por
	// roster.
	Attended int64
}

// findGroupRosterWithAttendanceSQL arma el LEFT JOIN de la grilla: el roster del
// grupo (miembros en la ventana de la fecha de la sesion, D7) cruzado con la
// asistencia de cada uno para esa sesion.
//
// El LEFT JOIN, no el INNER, es lo que hace aparecer a los corredores sin
// asistencia: con INNER la grilla solo mostraria los confirmados y el entrenador
// no podria marcar a los demas.
//
// roster_size NO se trae de aca: es la cantidad de filas de este SELECT, y se
// cuenta en Go (ver FindGroupRosterWithAttendance).
const findGroupRosterWithAttendanceSQL = `
SELECT
	gu.user_id        AS user_id,
	a.id              AS attendance_id,
	a.source          AS source,
	a.created_at      AS created_at
FROM group_users gu
JOIN groups g ON g.id = gu.group_id AND g.team_id = ?
LEFT JOIN attendances a
	ON a.training_session_id = ?
	AND a.user_id = gu.user_id
	AND a.team_id = ?
WHERE gu.group_id = ?
  AND gu.deleted_at IS NULL
  AND gu.date_start::date <= ?::date
  AND (gu.date_end IS NULL OR gu.date_end::date >= ?::date)
ORDER BY gu.user_id`

// countSessionAttendancesSQL cuenta las asistencias de la sesion sin filtrar por
// roster. Es una query aparte a proposito (ver FindGroupRosterWithAttendance).
const countSessionAttendancesSQL = `
SELECT COUNT(*)
FROM attendances
WHERE training_session_id = ? AND team_id = ?`

// FindGroupRosterWithAttendance devuelve las filas de la grilla y los dos
// conteos crudos que necesita el summary.
//
// Son DOS queries y no una a proposito: el agregado no se puede arrastrar en la
// misma fila que el roster cuando el roster esta vacio. Un SELECT con el
// agregado como columna devolveria cero filas, y attended pasaria a 0 aunque la
// sesion tuviera asistencias de gente que ya no figura en el grupo. Que es
// exactamente el caso que el max(0, roster_size - attended) de la spec esta
// Cubierto para tolerar, asi que leerlo de la misma query que las filas lo
// romperia.
//
// La segunda query es un COUNT(*) sobre un indice (idx_att_team_session), no un
// segundo barrido del roster.
//
// El orden de las filas no se resuelve aca: las grillas se ordenan por nombre
// (que se arma en el service con el batch lookup de usuarios), no por user_id.
func (d *attendanceDao) FindGroupRosterWithAttendance(ctx *gin.Context, groupID, teamID, sessionInstanceID int64, sessionDate time.Time) ([]SessionAttendanceRow, *SessionAttendanceAggregates, error) {
	var raw []struct {
		UserID       int64      `gorm:"column:user_id"`
		AttendanceID *int64     `gorm:"column:attendance_id"`
		Source       *string    `gorm:"column:source"`
		CreatedAt    *time.Time `gorm:"column:created_at"`
	}
	row := d.DB.Raw(findGroupRosterWithAttendanceSQL, teamID, sessionInstanceID, teamID, groupID, sessionDate, sessionDate).Scan(&raw)
	if row.Error != nil {
		return nil, nil, fmt.Errorf("error finding group roster with attendance: %w", row.Error)
	}

	rows := make([]SessionAttendanceRow, 0, len(raw))
	for _, r := range raw {
		rows = append(rows, SessionAttendanceRow{
			UserID:       r.UserID,
			AttendanceID: r.AttendanceID,
			Source:       r.Source,
			RegisteredAt: r.CreatedAt,
		})
	}

	var attended int64
	if err := d.DB.Raw(countSessionAttendancesSQL, sessionInstanceID, teamID).Scan(&attended).Error; err != nil {
		return nil, nil, fmt.Errorf("error counting session attendances: %w", err)
	}

	return rows, &SessionAttendanceAggregates{
		RosterSize: int64(len(rows)),
		Attended:   attended,
	}, nil
}

// bulkUpsertManualSQL hace el alta masiva en UNA sola sentencia: inserta las
// filas que faltan y actualiza las que ya existen, sin SELECT previo y sin
// riesgo de carrera entre dos requests del entrenador.
//
// El `ON CONFLICT` nombra explícitamente las tres columnas del índice único
// (team_id, training_session_id, user_id). Si ese índice no existiera con ese
// nombre/columnas, Postgres no fallaría con un error de constraint en runtime
// —por eso la tarea 3.7 pide un test DAO que lo verifique ruidosamente (R1).
//
// `RETURNING (xmax = 0) AS inserted` es la forma idiomática de distinguir un
// INSERT de un UPDATE en Postgres: en un INSERT recién hecho xmax vale 0, y en una
// fila actualizada por otro comando vale el xid de esa transacción. Es lo que
// permite contar created y updated sin una segunda consulta.
const bulkUpsertManualSQL = `
INSERT INTO attendances (team_id, training_session_id, user_id, source, registered_by_user_id, created_at, updated_at)
SELECT $1, $2, u.user_id, $3, $4, NOW(), NOW()
FROM unnest($5::bigint[]) AS u(user_id)
ON CONFLICT (team_id, training_session_id, user_id)
DO UPDATE SET
	updated_at = NOW(),
	source = EXCLUDED.source,
	registered_by_user_id = EXCLUDED.registered_by_user_id
RETURNING (xmax = 0) AS inserted`

// BulkUpsertManual da de alta la asistencia de varios corredores a una misma
// sesión en una sola sentencia. Es idempotente: reenviar el mismo lote no crea
// duplicados, refresca source y registered_by_user_id, y devuelve cuantos filas se
// insertaron y cuantas se actualizaron (contadores excluyentes).
//
// Un lote vacío devuelve (0, 0, nil) sin tocar la DB: es un no-op, no un error.
//
// La validación de que los userIDs son miembros del grupo NO se hace acá — es una
// regla de negocio y vive en el service, que la ejecuta ANTES de llamar a este
// método para poder rechazar el lote entero sin haber escrito nada.
func (d *attendanceDao) BulkUpsertManual(ctx *gin.Context, teamID, sessionInstanceID, actorUserID int64, userIDs []int64) (created int, updated int, err error) {
	if len(userIDs) == 0 {
		return 0, 0, nil
	}

	var results []struct {
		Inserted bool `gorm:"column:inserted"`
	}
	// El orden de los argumentos sigue la numeración $1..$5 del SQL de arriba.
	row := d.DB.Raw(bulkUpsertManualSQL, teamID, sessionInstanceID, string(constants.AttendanceSourceManual), actorUserID, userIDs).Scan(&results)
	if row.Error != nil {
		return 0, 0, fmt.Errorf("error bulk upserting attendances: %w", explainBulkUpsertError(row.Error))
	}

	for _, r := range results {
		if r.Inserted {
			created++
		} else {
			updated++
		}
	}
	return created, updated, nil
}

// FindByID devuelve la asistencia por id, o (nil, nil) si no existe.
func (d *attendanceDao) FindByID(ctx *gin.Context, attendanceID int64) (*dbs.Attendance, error) {
	var attendance dbs.Attendance
	row := d.DB.Where("id = ?", attendanceID).First(&attendance)
	if row.Error != nil {
		if errors.Is(row.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding attendance: %w", row.Error)
	}
	return &attendance, nil
}

// DeleteByID borra la asistencia si pertenece al equipo indicado, y devuelve si
// efectivamente borró.
//
// El team_id va en el WHERE y no solo en un chequeo previo: es la condición de
// seguridad en la propia sentencia, así que una fila de otro equipo no se borra
// ni por error de lógica aguas arriba. El borrado es físico (D4).
func (d *attendanceDao) DeleteByID(ctx *gin.Context, attendanceID, teamID int64) (bool, error) {
	row := d.DB.Where("id = ? AND team_id = ?", attendanceID, teamID).Delete(&dbs.Attendance{})
	if row.Error != nil {
		return false, fmt.Errorf("error deleting attendance: %w", row.Error)
	}
	return row.RowsAffected > 0, nil
}

// explainBulkUpsertError traduce el error de Postgres más probable de este método
// a un mensaje que dice qué revisar.
//
// Existe por el caso de arranque: si la base no tiene un índice UNIQUE no-parcial
// sobre (team_id, training_session_id, user_id), Postgres rechaza el ON CONFLICT
// con un mensaje que nombra la restricción pero no el modelo. Sin esto, quien lo
// lea en un 500 tiene que saber que el problema es el índice; con esto, lo dice.
//
// El match es por texto y no por SQLSTATE a propósito: no tengo el código exacto
// verificado para este mensaje, y un texto que no matchea degrada al error original
// (que sigue siendo correcto), mientras que un SQLSTATE inventado produciría un
// diagnóstico equivocado con seguridad.
func explainBulkUpsertError(err error) error {
	msg := err.Error()
	if !strings.Contains(msg, "ON CONFLICT") || !strings.Contains(msg, "unique or exclusion constraint") {
		return err
	}
	return fmt.Errorf(
		"%w | REVISAR LA MIGRACIÓN: attendances necesita un índice UNIQUE no-parcial sobre "+
			"(team_id, training_session_id, user_id) para el upsert. Crear con: "+
			"CREATE UNIQUE INDEX uq_att_team_session_user ON attendances (team_id, training_session_id, user_id). "+
			"Ojo: un índice PARCIAL (con WHERE) no sirve, y el nombre del índice es irrelevante: "+
			"Postgres matchea por columnas",
		err)
}
