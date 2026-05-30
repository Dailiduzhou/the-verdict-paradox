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
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // 使用人类可读的时间格式

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(os.Stdout),
		),
		zap.InfoLevel,
	)

	zlogger := zap.New(core)

	// 3. 包装为 Kratos 的 Logger
	logger := kratoszap.NewLogger(zlogger)

	// 4. 注入全局公共字段（TraceID 是排查问题的核心）
	return log.With(logger,
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.name", "the-verdict-paradox",
		"trace.id", tracing.TraceID(), // 自动从 context 提取 traceID
		"span.id", tracing.SpanID(),
	)
}
