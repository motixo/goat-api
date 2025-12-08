package logger

import (
	"go.uber.org/zap"
)

type ZapLogger struct {
	l *zap.SugaredLogger
}

func NewZapLogger() (*ZapLogger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return &ZapLogger{l: logger.Sugar()}, nil
}

func (z *ZapLogger) Info(msg string, fields ...any)  { z.l.Infow(msg, fields...) }
func (z *ZapLogger) Error(msg string, fields ...any) { z.l.Errorw(msg, fields...) }
func (z *ZapLogger) Warn(msg string, fields ...any)  { z.l.Warnw(msg, fields...) } // ← Fix this too!
func (z *ZapLogger) Debug(msg string, fields ...any) { z.l.Debugw(msg, fields...) }
func (z *ZapLogger) Panic(msg string, fields ...any) { z.l.Panicw(msg, fields...) }
