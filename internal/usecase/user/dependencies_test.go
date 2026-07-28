package user

import "testing"

func TestNewUsecaseMapsNamedDependencies(t *testing.T) {
	userRecorder := &deletionRecorder{}
	passwordRecorder := &passwordChangeRecorder{}

	dependencies := Dependencies{
		UserRepository:               &deletionUserRepository{recorder: userRecorder},
		DetailReader:                 &recordingUserDetailReader{},
		ListReader:                   &recordingUserListReader{},
		StatusReader:                 &recordingUserStatusSnapshotReader{},
		UpdateWriter:                 &panicOnGenericUpdateFullUserRepository{},
		EmailWriter:                  &recordingUserEmailWriter{},
		RoleWriter:                   &roleChangeWriter{},
		PasswordHasher:               &passwordChangeHasher{recorder: passwordRecorder},
		Logger:                       discardUserLogger{},
		SessionRepository:            &deletionSessionRepository{recorder: userRecorder},
		PasswordChangeCleanupMetrics: &passwordChangeCleanupMetrics{},
	}

	usecase, ok := NewUsecase(dependencies).(*UserUseCase)
	if !ok {
		t.Fatalf("NewUsecase() type = %T, want *UserUseCase", NewUsecase(dependencies))
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{name: "user repository", got: usecase.userRepo, want: dependencies.UserRepository},
		{name: "detail reader", got: usecase.userDetailReader, want: dependencies.DetailReader},
		{name: "list reader", got: usecase.userListReader, want: dependencies.ListReader},
		{name: "status reader", got: usecase.userStatusReader, want: dependencies.StatusReader},
		{name: "update writer", got: usecase.userUpdateWriter, want: dependencies.UpdateWriter},
		{name: "email writer", got: usecase.userEmailWriter, want: dependencies.EmailWriter},
		{name: "role writer", got: usecase.userRoleWriter, want: dependencies.RoleWriter},
		{name: "password hasher", got: usecase.passwordHasher, want: dependencies.PasswordHasher},
		{name: "logger", got: usecase.logger, want: dependencies.Logger},
		{name: "session repository", got: usecase.sessionRepo, want: dependencies.SessionRepository},
		{
			name: "password cleanup metrics",
			got:  usecase.metrics,
			want: dependencies.PasswordChangeCleanupMetrics,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("mapped dependency = %T, want %T", test.got, test.want)
			}
		})
	}
}
