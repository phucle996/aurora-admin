package logger

import (
	"admin/internal/config"
	appctxKey "admin/internal/key"
	"bytes"
	"context"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	accessLogType      = "ACCESSLOG"
	fieldLevelOverride = "_level_text"
)

var l = logrus.New()

// InitLogger configures the shared logrus logger.
func InitLogger(cfg *config.AppCfg) {
	l.SetOutput(os.Stdout)
	l.SetReportCaller(false)

	// Optional log level via LOG_LEVEL (defaults to Info on parse failure or empty).
	level := strings.ToLower(strings.TrimSpace(cfg.LogLV))
	if parsed, err := logrus.ParseLevel(level); err == nil {
		l.SetLevel(parsed)
	}

	l.SetFormatter(&customFormatter{
		base: &logrus.TextFormatter{
			FullTimestamp:          true,
			DisableLevelTruncation: true,
			PadLevelText:           true,
			DisableColors:          true,
		},
	})
}

// ===== INTERNAL =====
func newEntry(ctx context.Context, op string) *logrus.Entry {
	entry := l.WithContext(ctx)

	if strings.TrimSpace(op) != "" {
		entry = entry.WithField("op", op)
	}

	if rid, _ := ctx.Value(appctxKey.KeyRequestID).(string); rid != "" {
		entry = entry.WithField("req_id", rid)
	}

	return entry
}

// ===== HANDLER LOGS =====
func AccessLog(ctx context.Context, format string, args ...any) {
	newEntry(ctx, "").
		WithField(fieldLevelOverride, "ACCS").
		Infof(format, args...)
}

func HandlerInfo(ctx context.Context, op, format string, args ...any) {
	newEntry(ctx, op).
		WithField(fieldLevelOverride, "INFO").
		Infof(format, args...)
}

func HandlerDebug(ctx context.Context, op, format string, args ...any) {
	newEntry(ctx, op).
		WithField(fieldLevelOverride, "DEBG").
		Debugf(format, args...)
}

func HandlerWarn(ctx context.Context, op, format string, args ...any) {
	newEntry(ctx, op).
		WithField(fieldLevelOverride, "WARN").
		Warnf(format, args...)
}

func HandlerError(ctx context.Context, op string, err error, format string, args ...any) {
	newEntry(ctx, op).
		WithError(err).
		WithField(fieldLevelOverride, "EROR").
		Errorf(format, args...)
}

// ===== SYSTEM LOGS =====
func SysInfo(op, format string, args ...any) {
	entry := logrus.NewEntry(l)
	if strings.TrimSpace(op) != "" {
		entry = entry.WithField("op", op)
	}
	entry.WithField(fieldLevelOverride, "SYSF").Infof(format, args...)
}

func SysDebug(op, format string, args ...any) {
	entry := logrus.NewEntry(l)
	if strings.TrimSpace(op) != "" {
		entry = entry.WithField("op", op)
	}
	entry.WithField(fieldLevelOverride, "SYSD").Debugf(format, args...)
}

func SysWarn(op, format string, args ...any) {
	entry := logrus.NewEntry(l)
	if strings.TrimSpace(op) != "" {
		entry = entry.WithField("op", op)
	}
	entry.WithField(fieldLevelOverride, "SYSW").Warnf(format, args...)
}

func SysError(op string, err error, format string, args ...any) {
	entry := logrus.NewEntry(l)
	if strings.TrimSpace(op) != "" {
		entry = entry.WithField("op", op)
	}
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.WithField(fieldLevelOverride, "SYSE").Errorf(format, args...)
}

func SysFatal(op string, err error, format string, args ...any) {
	entry := logrus.NewEntry(l)
	if strings.TrimSpace(op) != "" {
		entry = entry.WithField("op", op)
	}
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.WithField(fieldLevelOverride, "SYSF").Fatalf(format, args...)
}

// ===== CUSTOM FORMATTER =====
type customFormatter struct {
	base *logrus.TextFormatter
}

func (f *customFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	levelText := strings.ToUpper(entry.Level.String())
	if override, ok := entry.Data[fieldLevelOverride].(string); ok && strings.TrimSpace(override) != "" {
		levelText = override
	}

	clone := *entry
	if len(entry.Data) > 0 {
		clone.Data = make(logrus.Fields, len(entry.Data))
		for k, v := range entry.Data {
			if k == fieldLevelOverride {
				continue
			}
			clone.Data[k] = v
		}
	}

	buf, err := f.base.Format(&clone)
	if err != nil {
		return nil, err
	}

	needle := "level=" + entry.Level.String()
	buf = bytes.Replace(buf, []byte(needle), []byte("level="+levelText), 1)
	needleUpper := "level=" + strings.ToUpper(entry.Level.String())
	buf = bytes.Replace(buf, []byte(needleUpper), []byte("level="+levelText), 1)

	return buf, nil
}
