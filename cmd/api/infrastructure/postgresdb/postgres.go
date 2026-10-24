package postgresdb

import (
	"fmt"
	"time"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gl "gorm.io/gorm/logger"
)

// ConnString arma el DSN de la app. Exportado para que el setup de test tome
// un advisory lock de migración sobre el mismo conn string.
func ConnString(configDB config.DB) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=%s",
		configDB.Host,
		configDB.Port,
		configDB.Username,
		configDB.Password,
		configDB.Name,
		time.UTC.String(),
	)
}

func ConfigDB(configDB config.DB) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(ConnString(configDB)), &gorm.Config{
		Logger: gl.Default.LogMode(gl.Silent),
	})

	if err != nil {
		customlogger.Error(nil, "cannot open postgres DB", err,
			customlogger.Tag("db_name", configDB.Name))
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		customlogger.Error(nil, "cannot get sql.DB instance", err,
			customlogger.Tag("db_name", configDB.Name))
		return nil, err
	}

	sqlDB.SetConnMaxLifetime(configDB.ConnMaxLifetime)
	sqlDB.SetMaxIdleConns(configDB.MaxIdleConnections)
	sqlDB.SetMaxOpenConns(configDB.MaxOpenConnections)

	err = sqlDB.Ping()
	if err != nil {
		customlogger.Error(nil, "error connecting to DB", err,
			customlogger.Tag("db_name", configDB.Name))
		return nil, err
	}

	customlogger.Info(nil, "DB connected successfully",
		customlogger.Tag("db_name", configDB.Name),
		customlogger.Tag("host", configDB.Host),
		customlogger.Tag("port", configDB.Port))

	stats := sqlDB.Stats()
	if stats.OpenConnections >= configDB.MaxOpenConnections {
		return nil, fmt.Errorf(
			"[DBNAME:%s] number of connections exceeded: %v",
			configDB.Name,
			stats.OpenConnections,
		)
	}

	err = db.AutoMigrate(
		&dbs.User{},
		&dbs.Permission{},
		&dbs.Role{},
		&dbs.Tier{},
		&dbs.TierPermission{},
		&dbs.UserRole{},
		&dbs.PasswordResetToken{},
		&dbs.Team{},
		&dbs.Group{},
		&dbs.TeamUser{},
		&dbs.GroupUser{},
		&dbs.Invitation{},
		&dbs.JoinRequest{},
		&dbs.RefreshToken{},
		&dbs.PushToken{},
		&dbs.Payment{},
		&dbs.UserRoleTierSubscription{},
		&dbs.Installment{},
		&dbs.SellerConnection{},
		&dbs.PlatformSetting{},
		&dbs.Attendance{},
		&dbs.Exercise{},
		&dbs.Session{},
		&dbs.SessionExercise{},
		&dbs.TrainingPlan{},
		&dbs.PlanDay{},
		&dbs.GroupCalendarDay{},
		&dbs.WorkoutFeedback{},
		&dbs.WorkoutFeedbackPoint{},
		&dbs.RunnerSession{},
		// Instancias de asignacion-por-instanciacion (design.md D1): copias
		// inmutables del catálogo, separadas de exercises/sessions.
		&dbs.ExerciseInstance{},
		&dbs.SessionInstance{},
		&dbs.SessionExerciseInstance{},
		// Mensajería de sesión (design.md D5): FK opaca al patrón del repo,
		// sin constraint física.
		&dbs.SessionMessage{},
		&dbs.SessionMessageRecipient{},
		// Gamificación (gamificacion-core E1): unseen + estado de racha.
		&dbs.GamificationUnseen{},
		&dbs.GamificationStreakState{},
		// Gamificación (gamificacion-core E2): progreso de logros + estampas.
		&dbs.UserAchievementProgress{},
		&dbs.GamificationStamp{},
		// Gamificación (gamificacion-core E4.1): competencias.
		&dbs.Competition{},
		&dbs.CompetitionParticipant{},
		&dbs.CompetitionWinner{},
	)
	if err != nil {
		customlogger.Error(nil, "auto-migrate failed", err)
		return nil, err
	}

	// Constraints que GORM no expresa por tags: se crean con SQL crudo post-
	// AutoMigrate, idempotente (vuelve a correr sin error si ya existen).
	// 1. Suscripciones por (user_id, role_id): como máximo una `active` y como
	//    máximo una `first_payment_pending` (dos índices parciales). Esto permite
	//    la ventana de cambio de tier: la sub del tier actual sigue `active`
	//    mientras la nueva del tier destino está `first_payment_pending`; la
	//    vieja pasa a `ended` recién cuando la nueva se confirma como `active`
	//    (cuota #1 pagada). El índice viejo (uq_sub_ids_user_role_active) unificaba
	//    ambos estados y no admitía esa coexistencia — se reemplaza.
	if err := db.Exec(`DROP INDEX IF EXISTS uq_sub_ids_user_role_active;`).Error; err != nil {
		customlogger.Error(nil, "error dropping legacy tier subscription partial unique index", err)
		return nil, err
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_sub_ids_user_role_active
		ON user_role_tier_subscriptions (user_id, role_id)
		WHERE status = 'active';`).Error; err != nil {
		customlogger.Error(nil, "error creating partial unique index on active tier subscriptions", err)
		return nil, err
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_sub_ids_user_role_pending
		ON user_role_tier_subscriptions (user_id, role_id)
		WHERE status = 'first_payment_pending';`).Error; err != nil {
		customlogger.Error(nil, "error creating partial unique index on pending tier subscriptions", err)
		return nil, err
	}

	// 2. Arco exclusivo en installments: exactamente uno de subscription_id o team_id.
	if err := db.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_installments_exclusive_parent') THEN
			ALTER TABLE installments ADD CONSTRAINT chk_installments_exclusive_parent
			CHECK (num_nonnulls(subscription_id, team_id) = 1);
		END IF;
	END $$;`).Error; err != nil {
		customlogger.Error(nil, "error creating exclusive parent check on installments", err)
		return nil, err
	}

	// 3. FK de payments.installment_id -> installments.id (columna aditiva sobre
	// tabla existente, no rompe nada; reintentable porque está en un DO guardado).
	if err := db.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_payments_installment') THEN
			ALTER TABLE payments ADD CONSTRAINT fk_payments_installment
			FOREIGN KEY (installment_id) REFERENCES installments(id) ON DELETE SET NULL;
		END IF;
	END $$;`).Error; err != nil {
		customlogger.Error(nil, "error creating payments installment FK", err)
		return nil, err
	}

	// 4. seller_connections: la clave única pasa de (user_id) a (user_id, client_id).
	// El access token OAuth es válido solo para la app que lo emitió; al versionar por
	// client_id evitamos que un reconnect contra otra app pise la conexión vigente.
	// Idempotente: columna recién agregada se completa con default '', el índice viejo
	// (si existía de un schema anterior) se dropea para liberar el user_id.
	// Se ejecutan por separado: el driver prepara los statements y no acepta
	// varios comandos en un solo Exec.
	migSellerConn := []string{
		`ALTER TABLE seller_connections ADD COLUMN IF NOT EXISTS client_id text NOT NULL DEFAULT '';`,
		`DROP INDEX IF EXISTS idx_seller_connections_user_id;`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_seller_connections_user_client
			ON seller_connections (user_id, client_id);`,
	}
	for _, stmt := range migSellerConn {
		if err := db.Exec(stmt).Error; err != nil {
			customlogger.Error(nil, "error migrating seller_connections unique key", err)
			return nil, err
		}
	}

	// 5. workout_feedback: "un feedback activo por set" vía índice único parcial
	// (WHERE deleted_at IS NULL) — la spec original pedía UNIQUE plano, pero eso
	// bloquearía recrear un set tras su baja lógica; ver design.md del change
	// workout-feedback-api. Idempotente (IF NOT EXISTS).
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS unique_feedback_per_set
		ON workout_feedback (assigned_session_id, assigned_exercise_id,
		athlete_user_id, feedback_owner_user_id, set_number) WHERE deleted_at IS NULL;`).Error; err != nil {
		customlogger.Error(nil, "error creating partial unique index on workout_feedback", err)
		return nil, err
	}

	// 6. workout_feedback.rpe: CHECK 1..10 (doble control con la validación del service).
	if err := db.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_workout_feedback_rpe') THEN
			ALTER TABLE workout_feedback ADD CONSTRAINT chk_workout_feedback_rpe
			CHECK (rpe IS NULL OR (rpe >= 1 AND rpe <= 10));
		END IF;
	END $$;`).Error; err != nil {
		customlogger.Error(nil, "error creating rpe check on workout_feedback", err)
		return nil, err
	}

	// 6bis. workout_feedback_points: "un punto por posición de serie" vía índice
	// único — la idempotencia del bulk POST /workout-feedback/:id/points se apoya
	// en esto (INSERT ... ON CONFLICT DO NOTHING). Ver change
	// workout-feedback-gps-points. Idempotente (IF NOT EXISTS).
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_feedback_point_order
		ON workout_feedback_points (feedback_id, "order");`).Error; err != nil {
		customlogger.Error(nil, "error creating unique index on workout_feedback_points", err)
		return nil, err
	}

	// 7. presencial_time/default_time (single) -> *_from/*_to (par obligatorio,
	// to > from) — ver docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md 2026-09-19.
	// Las columnas viejas se dropean: no hay migración limpia de un único valor
	// a un rango (no se puede inferir el horario de fin a partir del de inicio).
	migPresencialTimeRange := []string{
		`ALTER TABLE group_calendar_days DROP COLUMN IF EXISTS presencial_time;`,
		`ALTER TABLE plan_days DROP COLUMN IF EXISTS default_time;`,
	}
	for _, stmt := range migPresencialTimeRange {
		if err := db.Exec(stmt).Error; err != nil {
			customlogger.Error(nil, "error dropping legacy presencial/default time columns", err)
			return nil, err
		}
	}

	// 8. asignacion-por-instanciacion (design.md D11): group_calendar_days.session_id
	// apuntaba al catálogo — sin backfill posible. La columna vieja se dropea
	// junto con su índice si lo hubiera; session_instance_id la reemplaza.
	if err := db.Exec(`ALTER TABLE group_calendar_days DROP COLUMN IF EXISTS session_id;`).Error; err != nil {
		customlogger.Error(nil, "error dropping legacy group_calendar_days.session_id", err)
		return nil, err
	}

	// 9. exercises / exercise_instances: kind original -> vigente (2026-10-12).
	//
	// El frontend reemplazó los 5 kinds originales por otros 5 que declaran
	// qué datos definen el ejercicio (liviano, elongacion, ritmo_continuo,
	// velocidad_tiempo, velocidad_distancia). Los originales SIGUEN siendo
	// válidos en la API —ver constants/exercise_kind.go— pero el frontend nuevo
	// no los reconoce, así que las filas existentes hay que moverlas.
	//
	// Se migran las DOS tablas: `exercises` es el catálogo editable, y
	// `exercise_instances` son copias congeladas de los días ya asignados (con
	// feedback ya reportado al usuario). Sin migrar estas últimas, el historial
	// ya completado se mostraría sin datos ni tipo en la app.
	//
	// Idempotente por construcción: el UPDATE solo matchea valores originales, y
	// los kinds vigentes no son destino de ningún mapeo, así que correrlo de nuevo
	// no cambia nada. Los valores salen de constants.LegacyExerciseKindMigrations
	// —no se escriben a mano acá— para que el SQL no pueda desincronizarse de las
	// constantes que valida el service.
	//
	// Ojo al leer los datos resultantes: `jogging` y `cruising` colapsan a
	// `ritmo_continuo`, y el frontend nuevo exige distancia Y tiempo para ese
	// tipo. Los ejercicios que estaban cargados solo con uno de los dos quedan
	// como filas que no cumplen la matriz nueva. La API no valida la matriz
	// (es validación del cliente), así que no las rechaza: se corrigen a mano
	// desde el catálogo.
	kindMigrationStmts := []string{}
	for _, table := range []string{"exercises", "exercise_instances"} {
		for legacy, current := range constants.LegacyExerciseKindMigrations() {
			kindMigrationStmts = append(kindMigrationStmts, fmt.Sprintf(
				`UPDATE %s SET kind = '%s' WHERE kind = '%s';`, table, current, legacy,
			))
		}
	}
	for _, stmt := range kindMigrationStmts {
		if err := db.Exec(stmt).Error; err != nil {
			customlogger.Error(nil, "error migrating exercise kind values", err)
			return nil, err
		}
	}

	// 10. Índices de las consultas analíticas del dashboard (design.md D15).
	//
	// CONCURRENTLY es viable porque ConfigDB no abre transacciones: todo lo de
	// arriba corre en autocommit. Envolver ESTE bloque en db.Transaction lo
	// rompe (CONCURRENTLY no se permite dentro de una transacción) — si alguna
	// vez se necesita transactions para otra migración, va en su propio item.
	//
	// idx_runner_session_athlete: el UNIQUE vigente arranca por
	// session_instance_id, así que solo sirve para "la corrida de esta sesión".
	// La vista de corredor de un grupo filtra por corredor sin saber la sesión.
	//
	// idx_feedback_session_date: el dashboard filtra por sesión Y rango de
	// fechas, y el índice vigente de assigned_session_id no trae session_date
	// (el rango se filtraría en memoria, después del join).
	//
	// Un índice que queda INVALID a mitad de una falla NO lo reintenta el IF
	// NOT EXISTS: hay que dropearlo a mano. Rollback:
	//   DROP INDEX CONCURRENTLY IF EXISTS idx_runner_session_athlete;
	//   DROP INDEX CONCURRENTLY IF EXISTS idx_feedback_session_date;
	dashboardIndexes := []string{
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_runner_session_athlete
			ON runner_session (athlete_user_id);`,
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_feedback_session_date
			ON workout_feedback (assigned_session_id, session_date)
			WHERE deleted_at IS NULL;`,
	}
	for _, stmt := range dashboardIndexes {
		if err := db.Exec(stmt).Error; err != nil {
			customlogger.Error(nil, "error creating dashboard analytics index", err)
			return nil, err
		}
	}

	// 11. Backfill de exercise_instances.source_exercise_id (2026-10-13).
	//
	// La columna la agregó AutoMigrate en el change instancia-referencia-catalogo
	// (8670ffe, 2026-09-21) y se puebla al instanciar. AutoMigrate no toca filas
	// existentes, así que TODA instancia estampada antes de esa fecha quedó en
	// NULL. Sin el link, el dashboard no puede agrupar series: dos instancias de
	// "Distancia 4 km" de enero y marzo serían dos ejercicios distintos y la
	// progresión longitudinal se perdería.
	//
	// Es un backfill y no un fallback en cada query a propósito: el dato debería
	// estar escrito, y matchear por nombre en cada request cuesta y decide la
	// ambigüedad en silencio. Acá se matchea una vez, por (owner_id, nombre
	// normalizado), y lo que no resuelve queda en NULL para el fallback —que sí
	// es el caso raro y por eso está contado en data_quality.legacy_sessions.
	//
	// Idempotente: el UPDATE filtra las NULL, así que una segunda pasada no
	// toca nada. Corre en autocommit como el resto del bloque; si alguna vez
	// necesita transacción, va en su propio item (ver la nota del 10).
	if err := BackfillSourceExerciseID(db); err != nil {
		customlogger.Error(nil, "error backfilling source_exercise_id", err)
		return nil, err
	}

	customlogger.Info(nil, "DB initialized successfully",
		customlogger.Tag("db_name", configDB.Name))

	return db, nil
}
