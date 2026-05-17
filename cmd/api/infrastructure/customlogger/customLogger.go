package customlogger

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type LogLevel string

const (
	PanicLevel LogLevel = "panic"
	FatalLevel LogLevel = "fatal"
	ErrorLevel LogLevel = "error"
	WarnLevel  LogLevel = "warn"
	InfoLevel  LogLevel = "info"
	DebugLevel LogLevel = "debug"
	TraceLevel LogLevel = "trace"

	_callerTag        = "caller-line"
	_callerFileBase   = 4
	_tagMsgFormat     = "%s - %s"
	_XrequestID       = "x-request-id"
	_Flow             = "flow"
	_root             = "/"
	_currentFile      = "customlogger.go"
	_splitValue       = 2
	_splittedCount    = 2
)

var (
	TLogger *customlogger
	once    sync.Once
)

type customlogger struct {
	log           *logrus.Logger
	showFileName  bool
	showRequestID bool
	showFlow      bool
}

func init() {
	once.Do(func() {
		logrusLogger := &logrus.Logger{
			Out:       os.Stdout,
			Hooks:     make(logrus.LevelHooks),
			Level:     logrus.DebugLevel,
			Formatter: &logrus.TextFormatter{DisableColors: true, DisableSorting: true},
		}
		TLogger = &customlogger{
			log: logrusLogger,
		}
	})
}

func CustomConfig(logLevel LogLevel, fileNameFlag bool, requestIDFlag bool, flow bool) {
	setLogLevel(logLevel)
	showFileName(fileNameFlag)
	showRequestID(requestIDFlag)
	showFlow(flow)
}

func showRequestID(show bool) {
	TLogger.showRequestID = show
	TLogger.log.Infof("Show request-id : %t", show)
}

func showFlow(show bool) {
	TLogger.showFlow = show
	TLogger.log.Infof("Show flow: %t", show)
}

func setLogLevel(logLevel LogLevel) {
	if level, err := logrus.ParseLevel(string(logLevel)); err != nil {
		TLogger.log.Errorf("loglevel not found: %v", err)
		panic(err)
	} else {
		TLogger.log.Infof("loglevel: %s", logLevel)
		TLogger.log.Level = level
	}
}

func showFileName(show bool) {
	TLogger.showFileName = show
	TLogger.log.Infof("Show file name: %t", show)
}

func TagMethod(value string) string {
	return Tag("method", value)
}

func Tag(key string, value interface{}) string {
	return fmt.Sprintf("%s:%v", key, value)
}

func addRequestIDToTags(ctx *gin.Context, tags *[]string) {
	if ctx == nil {
		return
	}
	requestID := ctx.Writer.Header().Get("X-Request-Id")
	if requestID != "" {
		b := []string{_XrequestID + ":" + requestID}
		*tags = append(b, *tags...)
	}
}

func addFileNameToTags(tags *[]string) {
	caller := callerFileName()
	if caller != "" {
		b := []string{_callerTag + ":" + caller}
		*tags = append(b, *tags...)
	}
}

func buildLogEntry(ctx *gin.Context, tags []string, message string) (*logrus.Entry, string) {
	if TLogger.showRequestID {
		addRequestIDToTags(ctx, &tags)
	}
	if TLogger.showFileName {
		addFileNameToTags(&tags)
	}

	fields, err := getFields(tags)
	if err != nil {
		message = fmt.Sprintf(_tagMsgFormat, message, err.Error())
	}

	return TLogger.log.WithFields(fields), message
}

func getFields(tags []string) (logrus.Fields, error) {
	fields := make(logrus.Fields)
	wrongTags := []string{}
	var err error

	for _, value := range tags {
		values := strings.SplitN(value, ":", _splitValue)
		if len(values) < _splittedCount {
			wrongTags = append(wrongTags, value)
			continue
		}
		valueParsed := strings.ReplaceAll(values[1], "_", "-")
		fields[strings.TrimSpace(values[0])] = strings.TrimSpace(valueParsed)
	}

	if len(wrongTags) > 0 {
		err = fmt.Errorf("error parsing tags (%s)", strings.Join(wrongTags, ","))
	}

	return fields, err
}

func Print(e interface{}) {
	TLogger.log.Printf("%s", e)
}

func Printf(s string, args ...interface{}) {
	TLogger.log.Printf(s, args...)
}

func Debug(ctx *gin.Context, message string, tags ...string) {
	if TLogger.log.Level >= logrus.DebugLevel {
		tags = append(tags, "level:debug")
		entry, msg := buildLogEntry(ctx, tags, message)
		entry.Debug(msg)
	}
}

func Info(ctx *gin.Context, message string, tags ...string) {
	if TLogger.log.Level >= logrus.InfoLevel {
		tags = append(tags, "level:info")
		entry, msg := buildLogEntry(ctx, tags, message)
		entry.Info(msg)
	}
}

func Warn(ctx *gin.Context, message string, tags ...string) {
	if TLogger.log.Level >= logrus.WarnLevel {
		tags = append(tags, "level:warn")
		entry, msg := buildLogEntry(ctx, tags, message)
		entry.Warn(msg)
	}
}

func Error(ctx *gin.Context, message string, err error, tags ...string) {
	if TLogger.log.Level >= logrus.ErrorLevel {
		tags = append(tags, "level:error")
		msg := fmt.Sprintf("%s - ERROR: %v", message, err)
		entry, msg := buildLogEntry(ctx, tags, msg)
		entry.Error(msg)
	}
}

func Panic(ctx *gin.Context, message string, err error, tags ...string) {
	if TLogger.log.Level >= logrus.PanicLevel {
		tags = append(tags, "level:panic")
		msg := fmt.Sprintf("%s - PANIC: %v", message, err)
		entry, msg := buildLogEntry(ctx, tags, msg)
		entry.Panic(msg)
	}
}

func Debugf(ctx *gin.Context, format string, args ...interface{}) {
	if TLogger.log.Level >= logrus.DebugLevel {
		Debug(ctx, fmt.Sprintf(format, args...))
	}
}

func Infof(ctx *gin.Context, format string, args ...interface{}) {
	if TLogger.log.Level >= logrus.InfoLevel {
		Info(ctx, fmt.Sprintf(format, args...))
	}
}

func Warnf(ctx *gin.Context, format string, args ...interface{}) {
	if TLogger.log.Level >= logrus.WarnLevel {
		Warn(ctx, fmt.Sprintf(format, args...))
	}
}

func Errorf(ctx *gin.Context, format string, err error, args ...interface{}) {
	if TLogger.log.Level >= logrus.ErrorLevel {
		Error(ctx, fmt.Sprintf(format, args...), err)
	}
}

func Panicf(ctx *gin.Context, format string, err error, args ...interface{}) {
	if TLogger.log.Level >= logrus.PanicLevel {
		Panic(ctx, fmt.Sprintf(format, args...), err)
	}
}

func GetOut() io.Writer {
	return TLogger.log.Out
}

func callerFileName() string {
	retries := 5
	file := ""
	line := 0
	ok := false
	callerLevel := _callerFileBase

	for retries > 0 {
		_, file, line, ok = runtime.Caller(callerLevel)
		if !ok {
			return ""
		}

		if strings.Contains(file, _currentFile) {
			callerLevel++
			retries--
		} else {
			retries = 0
		}
	}

	baseFile := file
	slash := strings.LastIndex(file, _root)

	if slash >= 0 {
		file = file[0 : slash-1]
		slash = strings.LastIndex(file, _root)
		file = baseFile[slash+1:]
	}

	return fmt.Sprintf("%s-%d", file, line)
}
