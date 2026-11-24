package log

import (
	"path/filepath"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	logger *logrus.Logger

	levels = map[string]logrus.Level{
		"PANIC": logrus.PanicLevel,
		"FATAL": logrus.FatalLevel,
		"ERROR": logrus.ErrorLevel,
		"WARN":  logrus.WarnLevel,
		"INFO":  logrus.InfoLevel,
		"DEBUG": logrus.DebugLevel,
		"TRACE": logrus.TraceLevel,
	}
)

func GetLogger() *logrus.Logger {
	return logger
}

func Init(path, level string, maxDay int) {
	level = strings.ToUpper(level)
	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
	})
	logger.SetLevel(levels[level])

	logger.SetOutput(NewWriter(path, viper.GetString("application.name"), maxDay))
}

func NewWriter(path, filePrefix string, maxDay int) *rotatelogs.RotateLogs {
	day := 24 * time.Hour
	writer, _ := rotatelogs.New(
		filepath.Join(path, "logs", filePrefix+"-%Y%m%d.log"),
		rotatelogs.WithMaxAge(time.Duration(maxDay)*day),
		rotatelogs.WithRotationTime(day),
	)
	return writer
}
