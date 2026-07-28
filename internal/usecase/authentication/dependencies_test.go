package authentication

import (
	"testing"
	"time"
)

func TestNewUsecaseMapsNamedDependencies(t *testing.T) {
	dependencies := Dependencies{
		UserRepository:      &securitySnapshotUserRepository{},
		SecurityStateReader: &securitySnapshotStateReader{},
		SessionUseCase:      &securitySnapshotSessionUseCase{},
		PasswordHasher:      &securitySnapshotPasswordHasher{},
		JWTService:          &securitySnapshotJWTService{},
		Logger:              discardAuthLogger{},
		AccessTTL:           AccessTTL(2 * time.Minute),
		RefreshTTL:          RefreshTTL(12 * time.Hour),
		SessionTTL:          SessionTTL(7 * 24 * time.Hour),
	}

	usecase, ok := NewUsecase(dependencies).(*AuthenticationUseCase)
	if !ok {
		t.Fatalf("NewUsecase() type = %T, want *AuthenticationUseCase", NewUsecase(dependencies))
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{name: "user repository", got: usecase.userRepo, want: dependencies.UserRepository},
		{name: "security state reader", got: usecase.securityStates, want: dependencies.SecurityStateReader},
		{name: "session use case", got: usecase.sessionUC, want: dependencies.SessionUseCase},
		{name: "password hasher", got: usecase.passwordHasher, want: dependencies.PasswordHasher},
		{name: "JWT service", got: usecase.jwtService, want: dependencies.JWTService},
		{name: "logger", got: usecase.logger, want: dependencies.Logger},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("mapped dependency = %T, want %T", test.got, test.want)
			}
		})
	}

	if usecase.accessTTL != time.Duration(dependencies.AccessTTL) {
		t.Fatalf("access TTL = %s, want %s", usecase.accessTTL, time.Duration(dependencies.AccessTTL))
	}
	if usecase.refreshTTL != time.Duration(dependencies.RefreshTTL) {
		t.Fatalf("refresh TTL = %s, want %s", usecase.refreshTTL, time.Duration(dependencies.RefreshTTL))
	}
	if usecase.sessionTTL != time.Duration(dependencies.SessionTTL) {
		t.Fatalf("session TTL = %s, want %s", usecase.sessionTTL, time.Duration(dependencies.SessionTTL))
	}
}
