package config

import (
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type Environment int

const (
	Development Environment = iota
	Stage
	Production

	_production    = "production"
	_goEnvironment = "GO_ENVIRONMENT"
	_prod          = "prod"
	_test          = "test"
	_env           = "environment"
	_localScope    = "LOCAL"
)

// stageFlag es el mecanismo explícito para pegarle a la base/storage de producción
// de Supabase. Todo lo demás (nada, cualquier otro valor, CI, go test) usa testing
// stage por default — a propósito, para que un Render mal configurado o un `go run`
// suelto nunca toquen producción por accidente. No es lo mismo que Environment/
// GetEnvironment(): eso gobierna cómo se carga la config (local/test/prod), esto
// gobierna A CUÁL proyecto de Supabase apunta.
const stageFlag = "--stage=production"

// IsProductionStage indica si el proceso arrancó explícitamente con --stage=production.
// Sin ese flag exacto, siempre es testing stage.
func IsProductionStage() bool {
	for _, arg := range os.Args[1:] {
		if arg == stageFlag {
			return true
		}
	}
	return false
}

type DB struct {
	Username           string
	Password           string
	Host               string
	Port               string
	Name               string
	MaxIdleConnections int
	MaxOpenConnections int
	ConnMaxLifetime    time.Duration
}

type MailerConfig struct {
	APIKey string
	From   string
}

type MercadoPago struct {
	AccessToken   string
	PublicKey     string
	WebhookSecret string
	WebhookURL    string
	CurrencyID    string
	OAuthClientID string
	OAuthClientSecret string
	OAuthRedirectURI string
}

var (
	MyDB                 DB
	JWTSecret            string
	JWTIssuer            string
	JWTAudience          string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	MyMailer             MailerConfig
	MyMP                 MercadoPago
	TokenEncryptionKey   string
)

func (d Environment) String() string {
	return [...]string{"dev", "stage", "prod"}[d]
}

func GetFromString(s string) Environment {
	switch s {
	case "prod":
		return Production
	case "stage":
		return Stage
	default:
		return Development
	}
}

func GetEnvironment() string {
	return os.Getenv(_env)
}

func IsProduction() bool {
	return strings.Contains(GetEnvironment(), _prod)
}

func IsTest() bool {
	return strings.Contains(GetEnvironment(), _test)
}

func GetTestContext() *gin.Context {
	gin.SetMode(gin.ReleaseMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return ctx
}

func init() {
	LoadValues()
}

func LoadValues() {
	godotenv.Load()

	if IsProduction() {
		initProd()
		return
	}

	if IsTest() {
		initTest()
		return
	}

	initLocal()
}

func initLocal() {
	loadDBConfig()
	loadMailerConfig()
	loadMercadoPagoConfig()
}

func initProd() {
	loadDBConfig()
	loadMailerConfig()
	loadMercadoPagoConfig()
}

func initTest() {
	loadDBConfig()
	loadMailerConfig()
	loadMercadoPagoConfig()
}

func loadDBConfig() {
	dbURL := stagedDatabaseURL()
	if dbURL != "" {
		MyDB = parseDatabaseURL(dbURL)
	} else {
		MyDB.Name = os.Getenv("db_name")
		MyDB.Host = os.Getenv("db_host")
		MyDB.Port = os.Getenv("db_port")
		MyDB.Username = os.Getenv("db_user")
		MyDB.Password = os.Getenv("db_password")
	}
	MyDB = LoadDBConfigDB(MyDB)
	JWTSecret = os.Getenv("JWT_SECRET")
	JWTIssuer = getEnvOrDefault("JWT_ISSUER", "paceron-backend")
	JWTAudience = getEnvOrDefault("JWT_AUDIENCE", "paceron-app")
	AccessTokenDuration = getDurationOrDefault("ACCESS_TOKEN_DURATION", 15*time.Minute)
	RefreshTokenDuration = getDurationOrDefault("REFRESH_TOKEN_DURATION", 30*24*time.Hour)
}

// stagedDatabaseURL resuelve qué proyecto de Supabase usar según IsProductionStage.
// Default siempre testing — production exige el flag explícito.
func stagedDatabaseURL() string {
	if IsProductionStage() {
		return os.Getenv("SUPABASE_PRODUCTION_DATABASE_URL")
	}
	return os.Getenv("SUPABASE_TESTING_DATABASE_URL")
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDurationOrDefault(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func loadMailerConfig() {
	MyMailer.APIKey = os.Getenv("RESEND_API_KEY")
	MyMailer.From = os.Getenv("RESEND_FROM_ADDRESS")
}

func loadMercadoPagoConfig() {
	MyMP.AccessToken = os.Getenv("MERCADOPAGO_ACCESS_TOKEN")
	MyMP.PublicKey = os.Getenv("MERCADOPAGO_PUBLIC_KEY")
	MyMP.WebhookSecret = os.Getenv("MERCADOPAGO_WEBHOOK_SECRET")
	MyMP.WebhookURL = getEnvOrDefault("MERCADOPAGO_WEBHOOK_URL", "")
	MyMP.CurrencyID = getEnvOrDefault("MERCADOPAGO_CURRENCY_ID", "ARS")
	MyMP.OAuthClientID = os.Getenv("MP_OAUTH_CLIENT_ID")
	MyMP.OAuthClientSecret = os.Getenv("MP_OAUTH_CLIENT_SECRET")
	MyMP.OAuthRedirectURI = os.Getenv("MP_OAUTH_REDIRECT_URI")
	TokenEncryptionKey = os.Getenv("TOKEN_ENCRYPTION_KEY")
}

func parseDatabaseURL(dbURL string) DB {
	u, err := url.Parse(dbURL)
	if err != nil {
		return DB{}
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "5432"
	}

	password, _ := u.User.Password()

	return DB{
		Username: u.User.Username(),
		Password: password,
		Host:     host,
		Port:     port,
		Name:     strings.TrimPrefix(u.Path, "/"),
	}
}

func LoadDBConfigDB(myDB DB) DB {
	myDB.MaxIdleConnections = 5
	myDB.MaxOpenConnections = 5
	myDB.ConnMaxLifetime = 3 * time.Minute
	return myDB
}
