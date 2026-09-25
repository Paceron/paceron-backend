package postgresdb

import (
	"fmt"
	"time"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gl "gorm.io/gorm/logger"
)

func ConfigDB(configDB config.DB) (*gorm.DB, error) {
	loc := time.UTC

	connString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=%s",
		configDB.Host,
		configDB.Port,
		configDB.Username,
		configDB.Password,
		configDB.Name,
		loc.String(),
	)

	db, err := gorm.Open(postgres.Open(connString), &gorm.Config{
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

	customlogger.Info(nil, "DB initialized successfully",
		customlogger.Tag("db_name", configDB.Name))

	return db, nil
}
