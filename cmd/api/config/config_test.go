package config

import (
	"os"
	"path/filepath"
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

func TestEnvBoolDefaultTrue(t *testing.T) {
	os.Unsetenv("MP_OAUTH_TEST_TOKEN")
	assert.True(t, envBoolDefaultTrue("MP_OAUTH_TEST_TOKEN"), "sin env var → default true")

	os.Setenv("MP_OAUTH_TEST_TOKEN", "true")
	assert.True(t, envBoolDefaultTrue("MP_OAUTH_TEST_TOKEN"))
	os.Setenv("MP_OAUTH_TEST_TOKEN", "TRUE")
	assert.True(t, envBoolDefaultTrue("MP_OAUTH_TEST_TOKEN"), "case-insensitive")
	os.Setenv("MP_OAUTH_TEST_TOKEN", "false")
	assert.False(t, envBoolDefaultTrue("MP_OAUTH_TEST_TOKEN"))
	os.Setenv("MP_OAUTH_TEST_TOKEN", "0")
	assert.False(t, envBoolDefaultTrue("MP_OAUTH_TEST_TOKEN"))
	os.Setenv("MP_OAUTH_TEST_TOKEN", "invalido")
	assert.True(t, envBoolDefaultTrue("MP_OAUTH_TEST_TOKEN"), "valor inválido → default true")
	os.Unsetenv("MP_OAUTH_TEST_TOKEN")
}

func TestLoadMailerConfig(t *testing.T) {
	os.Setenv("RESEND_API_KEY", "re_test_key")
	os.Setenv("RESEND_FROM_ADDRESS", "Paceron <no-reply@paceron.com>")
	defer func() {
		os.Unsetenv("RESEND_API_KEY")
		os.Unsetenv("RESEND_FROM_ADDRESS")
	}()

	loadMailerConfig()

	assert.Equal(t, "re_test_key", MyMailer.APIKey)
	assert.Equal(t, "Paceron <no-reply@paceron.com>", MyMailer.From)
}

func TestLoadMailerConfig_Empty(t *testing.T) {
	os.Unsetenv("RESEND_API_KEY")
	os.Unsetenv("RESEND_FROM_ADDRESS")

	loadMailerConfig()

	assert.Equal(t, "", MyMailer.APIKey)
	assert.Equal(t, "", MyMailer.From)
}

// withStage corre fn con os.Args seteado como si el proceso hubiera arrancado
// con ese flag, y lo restaura después. Los helpers de stage leen os.Args directo,
// así que no hay inyeccion posible sin tocar la variable global.
func withStage(t *testing.T, args []string, fn func()) {
	t.Helper()
	original := os.Args
	os.Args = args
	defer func() { os.Args = original }()
	fn()
}

func TestIsLocalStage_DefaultFalse(t *testing.T) {
	assert.False(t, IsLocalStage())
}

func TestIsLocalStage_WithFlag(t *testing.T) {
	withStage(t, []string{"paceron-backend", "--stage=local"}, func() {
		assert.True(t, IsLocalStage())
		assert.False(t, IsProductionStage())
	})
}

func TestIsLocalStage_WithUnrelatedFlags(t *testing.T) {
	withStage(t, []string{"paceron-backend", "-test.run=TestFoo", "--stage=local-debug"}, func() {
		assert.False(t, IsLocalStage(), "solo el flag exacto cuenta")
	})
}

// El stage local gana si vienen los dos flags: conectar a la base local es
// reversible, haber tocado produccion no lo es.
func TestIsLocalStage_WinsOverProduction(t *testing.T) {
	withStage(t, []string{"paceron-backend", "--stage=production", "--stage=local"}, func() {
		assert.True(t, IsLocalStage())
		assert.True(t, IsProductionStage(), "el flag de produccion sigue estando")
		assert.Equal(t, "S3_", stagedStorageEnvPrefix(), "pero la resolucion va a local")
	})
}

func TestStagedDatabaseURL_LocalReadsDatabaseURL(t *testing.T) {
	os.Setenv("SUPABASE_TESTING_DATABASE_URL", "postgresql://t:t@testhost:5432/testdb")
	os.Setenv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/paceron_local")
	defer func() {
		os.Unsetenv("SUPABASE_TESTING_DATABASE_URL")
		os.Unsetenv("DATABASE_URL")
	}()

	withStage(t, []string{"paceron-backend", "--stage=local"}, func() {
		assert.Equal(t, "postgresql://postgres:postgres@localhost:5432/paceron_local", stagedDatabaseURL())
	})
}

// Sin el flag local, DATABASE_URL no debe tener efecto: es el nombre que se
// revive para el stage local, no un override global.
func TestStagedDatabaseURL_IgnoresDatabaseURLWithoutLocalFlag(t *testing.T) {
	os.Setenv("SUPABASE_TESTING_DATABASE_URL", "postgresql://t:t@testhost:5432/testdb")
	os.Setenv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/paceron_local")
	defer func() {
		os.Unsetenv("SUPABASE_TESTING_DATABASE_URL")
		os.Unsetenv("DATABASE_URL")
	}()

	assert.Equal(t, "postgresql://t:t@testhost:5432/testdb", stagedDatabaseURL())
}

func TestStagedStorageEnvPrefix(t *testing.T) {
	withStage(t, []string{"paceron-backend", "--stage=local"}, func() {
		assert.Equal(t, "S3_", stagedStorageEnvPrefix())
	})
	withStage(t, []string{"paceron-backend", "--stage=production"}, func() {
		assert.Equal(t, "SUPABASE_PRODUCTION_S3_", stagedStorageEnvPrefix())
	})
	withStage(t, []string{"paceron-backend"}, func() {
		assert.Equal(t, "SUPABASE_TESTING_S3_", stagedStorageEnvPrefix())
	})
}

func TestLoadStorageConfig_Local(t *testing.T) {
	os.Setenv("S3_ENDPOINT", "http://localhost:9000")
	os.Setenv("S3_REGION", "us-east-1")
	os.Setenv("S3_ACCESS_ID", "minioadmin")
	os.Setenv("S3_SECRET_KEY", "minioadmin")
	os.Setenv("S3_BUCKET", "paceron-media")
	os.Setenv("S3_PUBLIC_BASE_URL", "http://localhost:9000/paceron-media")
	os.Setenv("S3_FORCE_PATH_STYLE", "true")
	defer func() {
		for _, k := range []string{"S3_ENDPOINT", "S3_REGION", "S3_ACCESS_ID", "S3_SECRET_KEY", "S3_BUCKET", "S3_PUBLIC_BASE_URL", "S3_FORCE_PATH_STYLE"} {
			os.Unsetenv(k)
		}
	}()

	withStage(t, []string{"paceron-backend", "--stage=local"}, loadStorageConfig)

	assert.Equal(t, "http://localhost:9000", MyStorage.Endpoint)
	assert.Equal(t, "us-east-1", MyStorage.Region)
	assert.Equal(t, "minioadmin", MyStorage.AccessKeyID)
	assert.Equal(t, "minioadmin", MyStorage.SecretAccessKey)
	assert.Equal(t, "paceron-media", MyStorage.Bucket)
	assert.Equal(t, "http://localhost:9000/paceron-media", MyStorage.PublicBaseURL)
	assert.True(t, MyStorage.ForcePathStyle)
}

// El prefijo S3_ no debe pisar el de testing: si alguien corre sin el flag local
// teniendo las variables S3_ en el entorno, tiene que seguir yendo a Supabase.
func TestLoadStorageConfig_LocalVarsIgnoredWithoutFlag(t *testing.T) {
	os.Setenv("S3_ENDPOINT", "http://localhost:9000")
	os.Setenv("SUPABASE_TESTING_S3_ENDPOINT", "https://testing.supabase.co/storage/v1/s3")
	defer func() {
		os.Unsetenv("S3_ENDPOINT")
		os.Unsetenv("SUPABASE_TESTING_S3_ENDPOINT")
	}()

	loadStorageConfig()

	assert.Equal(t, "https://testing.supabase.co/storage/v1/s3", MyStorage.Endpoint)
}

// S3_FORCE_PATH_STYLE y S3_PUBLIC_BASE_URL no van con prefijo de stage, asi que
// tienen que leerse igual en los tres ambientes.
func TestLoadStorageConfig_PathStyleDefaultsTrueInEveryStage(t *testing.T) {
	os.Unsetenv("S3_FORCE_PATH_STYLE")

	for _, args := range [][]string{
		{"paceron-backend"},
		{"paceron-backend", "--stage=production"},
		{"paceron-backend", "--stage=local"},
	} {
		withStage(t, args, loadStorageConfig)
		assert.True(t, MyStorage.ForcePathStyle, "args=%v", args)
	}

	os.Setenv("S3_FORCE_PATH_STYLE", "false")
	defer os.Unsetenv("S3_FORCE_PATH_STYLE")
	withStage(t, []string{"paceron-backend"}, loadStorageConfig)
	assert.False(t, MyStorage.ForcePathStyle)
}

func TestLoadStorageConfig_PathStyleFalseVariants(t *testing.T) {
	defer os.Unsetenv("S3_FORCE_PATH_STYLE")

	for _, value := range []string{"false", "FALSE", "0", "no", "No"} {
		os.Setenv("S3_FORCE_PATH_STYLE", value)
		withStage(t, []string{"paceron-backend"}, loadStorageConfig)
		assert.False(t, MyStorage.ForcePathStyle, "valor=%q", value)
	}
}

// Sin S3_PUBLIC_BASE_URL hay que seguir derivados de Supabase: ese es el
// comportamiento de siempre contra produccion y no puede cambiar.
func TestLoadStorageConfig_PublicBaseURLEmptyByDefault(t *testing.T) {
	os.Unsetenv("S3_PUBLIC_BASE_URL")

	for _, args := range [][]string{
		{"paceron-backend"},
		{"paceron-backend", "--stage=production"},
		{"paceron-backend", "--stage=local"},
	} {
		withStage(t, args, loadStorageConfig)
		assert.Equal(t, "", MyStorage.PublicBaseURL, "args=%v", args)
	}
}

func TestLoadValues_OverloadsDotEnvLocalOnLocalStage(t *testing.T) {
	// Regression: el backend solo llamaba godotenv.Load(), que lee .env. Entonces
	// .env.local — el mismo archivo que usa `docker compose --env-file` — era
	// inerte para el proceso, y S3_*/DATABASE_URL nunca llegaban.
	dir := t.TempDir()
	writeFile := func(name, content string) string {
		path := filepath.Join(dir, name)
		assert.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}
	writeFile(".env", "S3_ENDPOINT=https://testing.supabase.co/storage/v1/s3\nSHARED_ONLY=from-dotenv\n")
	writeFile(".env.local", "S3_ENDPOINT=http://localhost:9000\nS3_BUCKET=paceron-media\n")

	cwd, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(dir))
	defer func() {
		assert.NoError(t, os.Chdir(cwd))
		os.Unsetenv("S3_ENDPOINT")
		os.Unsetenv("S3_BUCKET")
		os.Unsetenv("SHARED_ONLY")
		os.Unsetenv("S3_PUBLIC_BASE_URL")
		os.Unsetenv("S3_FORCE_PATH_STYLE")
	}()

	withStage(t, []string{"paceron-backend", "--stage=local"}, LoadValues)

	// .env.local pisa a .env para las claves que define...
	assert.Equal(t, "http://localhost:9000", os.Getenv("S3_ENDPOINT"))
	// ...y suma las suyas.
	assert.Equal(t, "paceron-media", os.Getenv("S3_BUCKET"))
	// ...sin perder lo que .env tiene y .env.local no.
	assert.Equal(t, "from-dotenv", os.Getenv("SHARED_ONLY"))
}

func TestLoadValues_IgnoresDotEnvLocalWithoutLocalStage(t *testing.T) {
	// El otro riesgo: que .env.local pise la config de testing/produccion sin
	// haber pedido --stage=local.
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("S3_ENDPOINT=https://testing.supabase.co/storage/v1/s3\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, ".env.local"), []byte("S3_ENDPOINT=http://localhost:9000\n"), 0o600))

	cwd, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(dir))
	defer func() {
		assert.NoError(t, os.Chdir(cwd))
		os.Unsetenv("S3_ENDPOINT")
	}()

	LoadValues()

	assert.Equal(t, "https://testing.supabase.co/storage/v1/s3", os.Getenv("S3_ENDPOINT"))
}

// Un .env.local ausente no puede romper el arranque: godotenv.Overload no
// encuentra el archivo y solo lo reporta.
func TestLoadValues_MissingDotEnvLocalIsNotFatal(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("S3_ENDPOINT=https://testing.supabase.co/storage/v1/s3\n"), 0o600))

	cwd, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(dir))
	defer func() {
		assert.NoError(t, os.Chdir(cwd))
		os.Unsetenv("S3_ENDPOINT")
	}()

	assert.NotPanics(t, func() {
		withStage(t, []string{"paceron-backend", "--stage=local"}, LoadValues)
	})
}
