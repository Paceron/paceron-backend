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

var (
	MyDB      DB
	JWTSecret string
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
}

func initProd() {
	loadDBConfig()
}

func initTest() {
	loadDBConfig()
}

func loadDBConfig() {
	dbURL := os.Getenv("DATABASE_URL")
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
