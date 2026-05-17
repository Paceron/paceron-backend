package config

import (
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

var MyDB DB

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
	MyDB.Name = os.Getenv("db_name")
	MyDB.Host = os.Getenv("db_host")
	MyDB.Port = os.Getenv("db_port")
	MyDB.Username = os.Getenv("db_user")
	MyDB.Password = os.Getenv("db_password")
	MyDB = LoadDBConfigDB(MyDB)
}

func initProd() {
	MyDB.Name = os.Getenv("db_name")
	MyDB.Host = os.Getenv("db_host")
	MyDB.Port = os.Getenv("db_port")
	MyDB.Username = os.Getenv("db_user")
	MyDB.Password = os.Getenv("db_password")
	MyDB = LoadDBConfigDB(MyDB)
}

func initTest() {
	MyDB.Name = os.Getenv("db_name")
	MyDB.Host = os.Getenv("db_host")
	MyDB.Port = os.Getenv("db_port")
	MyDB.Username = os.Getenv("db_user")
	MyDB.Password = os.Getenv("db_password")
	MyDB = LoadDBConfigDB(MyDB)
}

func LoadDBConfigDB(myDB DB) DB {
	myDB.MaxIdleConnections = 5
	myDB.MaxOpenConnections = 5
	myDB.ConnMaxLifetime = 3 * time.Minute
	return myDB
}
