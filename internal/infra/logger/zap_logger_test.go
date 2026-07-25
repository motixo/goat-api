package logger

import "testing"

func TestZapLoggerSyncHandlesStandardStreamSync(t *testing.T) {
	t.Parallel()

	appLogger, err := NewZapLogger()
	if err != nil {
		t.Fatalf("NewZapLogger() error = %v", err)
	}
	if err := appLogger.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
}
