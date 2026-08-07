package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDatabaseURL(t *testing.T) {
	dbURL := "postgresql://user:pass@host:5432/dbname"
	db := parseDatabaseURL(dbURL)

	assert.Equal(t, "user", db.Username)
	assert.Equal(t, "pass", db.Password)
	assert.Equal(t, "host", db.Host)
	assert.Equal(t, "5432", db.Port)
	assert.Equal(t, "dbname", db.Name)
}

func TestParseDatabaseURL_SpecialCharsInPassword(t *testing.T) {
	dbURL := "postgresql://admin:p%40ss@localhost:5432/mydb"
	db := parseDatabaseURL(dbURL)

	assert.Equal(t, "admin", db.Username)
	assert.Equal(t, "p@ss", db.Password)
	assert.Equal(t, "localhost", db.Host)
	assert.Equal(t, "5432", db.Port)
	assert.Equal(t, "mydb", db.Name)
}

func TestParseDatabaseURL_DefaultPort(t *testing.T) {
	dbURL := "postgresql://user:pass@host/dbname"
	db := parseDatabaseURL(dbURL)

	assert.Equal(t, "host", db.Host)
	assert.Equal(t, "5432", db.Port)
	assert.Equal(t, "dbname", db.Name)
}

func TestParseDatabaseURL_InvalidURL(t *testing.T) {
	db := parseDatabaseURL("://invalid")
	assert.Equal(t, DB{}, db)
}

func TestLoadDBConfigDB(t *testing.T) {
	db := DB{}
	result := LoadDBConfigDB(db)

	assert.Equal(t, 5, result.MaxIdleConnections)
	assert.Equal(t, 5, result.MaxOpenConnections)
	assert.NotZero(t, result.ConnMaxLifetime)
}

func TestLoadValues_WithDatabaseURL(t *testing.T) {
	os.Setenv("SUPABASE_TESTING_DATABASE_URL", "postgresql://urluser:urlpass@urlhost:5555/urldb")
	defer os.Unsetenv("SUPABASE_TESTING_DATABASE_URL")

	loadDBConfig()

	assert.Equal(t, "urluser", MyDB.Username)
	assert.Equal(t, "urlpass", MyDB.Password)
	assert.Equal(t, "urlhost", MyDB.Host)
	assert.Equal(t, "5555", MyDB.Port)
	assert.Equal(t, "urldb", MyDB.Name)
	assert.Equal(t, 5, MyDB.MaxIdleConnections)
}

func TestLoadValues_WithIndividualVars(t *testing.T) {
	os.Unsetenv("SUPABASE_TESTING_DATABASE_URL")
	os.Unsetenv("SUPABASE_PRODUCTION_DATABASE_URL")
	os.Setenv("db_host", "indhost")
	os.Setenv("db_port", "7777")
	os.Setenv("db_user", "induser")
	os.Setenv("db_password", "indpass")
	os.Setenv("db_name", "inddb")
	defer func() {
		os.Unsetenv("db_host")
		os.Unsetenv("db_port")
		os.Unsetenv("db_user")
		os.Unsetenv("db_password")
		os.Unsetenv("db_name")
	}()

	loadDBConfig()

	assert.Equal(t, "induser", MyDB.Username)
	assert.Equal(t, "indpass", MyDB.Password)
	assert.Equal(t, "indhost", MyDB.Host)
	assert.Equal(t, "7777", MyDB.Port)
	assert.Equal(t, "inddb", MyDB.Name)
}

func TestIsProductionStage_DefaultFalse(t *testing.T) {
	assert.False(t, IsProductionStage())
}

func TestIsProductionStage_WithFlag(t *testing.T) {
	original := os.Args
	os.Args = []string{"paceron-backend", "--stage=production"}
	defer func() { os.Args = original }()

	assert.True(t, IsProductionStage())
}

func TestIsProductionStage_WithUnrelatedFlags(t *testing.T) {
	original := os.Args
	os.Args = []string{"paceron-backend", "-test.run=TestFoo", "-test.v"}
	defer func() { os.Args = original }()

	assert.False(t, IsProductionStage())
}

func TestStagedDatabaseURL_DefaultsToTesting(t *testing.T) {
	os.Setenv("SUPABASE_TESTING_DATABASE_URL", "postgresql://t:t@testhost:5432/testdb")
	os.Setenv("SUPABASE_PRODUCTION_DATABASE_URL", "postgresql://p:p@prodhost:5432/proddb")
	defer func() {
		os.Unsetenv("SUPABASE_TESTING_DATABASE_URL")
		os.Unsetenv("SUPABASE_PRODUCTION_DATABASE_URL")
	}()

	assert.Equal(t, "postgresql://t:t@testhost:5432/testdb", stagedDatabaseURL())
}

func TestStagedDatabaseURL_ProductionWithFlag(t *testing.T) {
	os.Setenv("SUPABASE_TESTING_DATABASE_URL", "postgresql://t:t@testhost:5432/testdb")
	os.Setenv("SUPABASE_PRODUCTION_DATABASE_URL", "postgresql://p:p@prodhost:5432/proddb")
	defer func() {
		os.Unsetenv("SUPABASE_TESTING_DATABASE_URL")
		os.Unsetenv("SUPABASE_PRODUCTION_DATABASE_URL")
	}()
	original := os.Args
	os.Args = []string{"paceron-backend", "--stage=production"}
	defer func() { os.Args = original }()

	assert.Equal(t, "postgresql://p:p@prodhost:5432/proddb", stagedDatabaseURL())
}

func TestLoadSMTPConfig(t *testing.T) {
	os.Setenv("SMTP_HOST", "smtp.gmail.com")
	os.Setenv("SMTP_PORT", "587")
	os.Setenv("GMAIL_USER", "test@gmail.com")
	os.Setenv("GMAIL_APP_PASSWORD", "app-password-value")
	defer func() {
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_PORT")
		os.Unsetenv("GMAIL_USER")
		os.Unsetenv("GMAIL_APP_PASSWORD")
	}()

	loadSMTPConfig()

	assert.Equal(t, "smtp.gmail.com", MySMTP.Host)
	assert.Equal(t, 587, MySMTP.Port)
	assert.Equal(t, "test@gmail.com", MySMTP.User)
	assert.Equal(t, "app-password-value", MySMTP.AppPassword)
}

func TestLoadSMTPConfig_DefaultPort(t *testing.T) {
	os.Unsetenv("SMTP_PORT")
	os.Setenv("SMTP_HOST", "smtp.gmail.com")
	defer os.Unsetenv("SMTP_HOST")

	loadSMTPConfig()

	assert.Equal(t, 587, MySMTP.Port)
}
