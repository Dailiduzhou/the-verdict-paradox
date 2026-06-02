package logger

import (
	"os"

	kratoszap "github.com/go-kratos/kratos/contrib/log/zap/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewJSONLogger() log.Logger {
	level := zap.InfoLevel
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		if parsed, err := zapcore.ParseLevel(lvl); err == nil {
			level = parsed
		}
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(os.Stdout),
		),
		level,
	)

	zlogger := zap.New(core)

	logger := kratoszap.NewLogger(zlogger)

	return log.With(logger,
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.name", "the-verdict-paradox",
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
}
