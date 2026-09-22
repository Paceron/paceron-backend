package testutils

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// FailingDB devuelve un handle *gorm.DB cuyo ConnPool inyecta un error de DB
// para las operaciones marcadas por fail, decididas por TIPO de operación
// ("select", "insert", "update", "delete"), nunca por texto del SQL. Sirve
// para cubrir las ramas de error de DB de servicios y DAOs sin acoplarse a
// mensajes de Postgres. El handle original queda intacto; SAVEPOINT/BEGIN se
// delegan (funciona con handles de transacción de SetupTestDB, cuyo
// s.db.Transaction usa savepoints).
func FailingDB(t *testing.T, db *gorm.DB, fail func(op string) bool) *gorm.DB {
	t.Helper()
	underlying := db.Statement.ConnPool
	pool := &failingConnPool{underlying: underlying, fail: fail}
	if committer, ok := underlying.(gorm.TxCommitter); ok {
		pool.committer = committer
	}
	// Context no-nil fuerza el Statement.clone() del Session y el handle
	// original no comparte el pool falsificado.
	sess := db.Session(&gorm.Session{Context: db.Statement.Context, NewDB: true})
	sess.Statement.ConnPool = pool
	return sess
}

// failingConnPool implementa gorm.ConnPool (+TxCommitter/TxBeginner cuando el
// pool base lo es) dejando pasar todo salvo los exec marcados.
type failingConnPool struct {
	underlying gorm.ConnPool
	committer  gorm.TxCommitter
	fail       func(op string) bool
}

var errInjected = errors.New("error de base de datos inyectado por testutils.FailingDB")

func opOfSQL(query string) string {
	for _, prefix := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
		if strings.HasPrefix(query, prefix) {
			return strings.ToLower(prefix)
		}
	}
	return "other"
}

func (p *failingConnPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if op := opOfSQL(query); op != "other" && p.fail(op) {
		return nil, errInjected
	}
	return p.underlying.ExecContext(ctx, query, args...)
}

func (p *failingConnPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if op := opOfSQL(query); op != "other" && p.fail(op) {
		return nil, errInjected
	}
	return p.underlying.QueryContext(ctx, query, args...)
}

func (p *failingConnPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if op := opOfSQL(query); op != "other" && p.fail(op) {
		// No hay forma de fabricar un *sql.Row fallido; la ruta .Row() de gorm
		// no es objetivo de los tests que usan este helper.
		return p.underlying.QueryRowContext(ctx, "SELECT 1 WHERE false")
	}
	return p.underlying.QueryRowContext(ctx, query, args...)
}

func (p *failingConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.underlying.PrepareContext(ctx, query)
}

func (p *failingConnPool) BeginTx(ctx context.Context, opt *sql.TxOptions) (*sql.Tx, error) {
	// Deliberadamente sin BeginTx: sobre handles de transacción (SetupTestDB),
	// el auto-begin de gorm para statementes Create/Update/Delete eleva
	// ErrInvalidTransaction, que gorm descarta y deja correr el statement con
	// este mismo pool (la inyección de fallo sigue activa).
	return nil, gorm.ErrInvalidTransaction
}

func (p *failingConnPool) Commit() error {
	if p.committer == nil {
		return errors.New("FailingDB sin committer")
	}
	return p.committer.Commit()
}

func (p *failingConnPool) Rollback() error {
	if p.committer == nil {
		return errors.New("FailingDB sin committer")
	}
	return p.committer.Rollback()
}
