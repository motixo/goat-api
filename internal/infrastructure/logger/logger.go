package logger

type Logger interface {
	Info(msg string, fields ...any)
	Error(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Debug(msg string, fields ...any)
	Panic(msg string, fields ...any)
}
