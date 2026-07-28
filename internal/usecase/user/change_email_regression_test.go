package user

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
)

func TestChangeEmailUsesFocusedWriterWithoutLoadingUserAggregate(t *testing.T) {
	writer := &recordingUserEmailWriter{}
	usecase := NewUsecase(Dependencies{
		UserRepository: &panicOnChangeEmailFullUserRepository{},
		EmailWriter:    writer,
		Logger:         discardUserLogger{},
	})

	input := UpdateEmailInput{UserID: "user-1", Email: "new@example.com"}
	if err := usecase.ChangeEmail(context.Background(), input); err != nil {
		t.Fatalf("ChangeEmail() error = %v", err)
	}

	want := UserEmailUpdateCommand{UserID: input.UserID, Email: input.Email}
	if writer.calls != 1 || !reflect.DeepEqual(writer.command, want) {
		t.Fatalf("email writer calls/command = %d/%#v, want 1/%#v", writer.calls, writer.command, want)
	}
}

func TestChangeEmailPreservesFocusedWriterError(t *testing.T) {
	persistenceErr := errors.New("postgres email update failed")
	writer := &recordingUserEmailWriter{err: persistenceErr}
	usecase := NewUsecase(Dependencies{
		UserRepository: &panicOnChangeEmailFullUserRepository{},
		EmailWriter:    writer,
		Logger:         discardUserLogger{},
	})

	err := usecase.ChangeEmail(context.Background(), UpdateEmailInput{
		UserID: "user-1",
		Email:  "new@example.com",
	})
	if !errors.Is(err, persistenceErr) {
		t.Fatalf("ChangeEmail() error = %v, want preserved persistence error", err)
	}
}

type panicOnChangeEmailFullUserRepository struct {
	repository.UserRepository
}

func (*panicOnChangeEmailFullUserRepository) FindByID(
	context.Context,
	string,
) (*entity.User, error) {
	panic("ChangeEmail must not load a full User aggregate")
}

type recordingUserEmailWriter struct {
	calls   int
	command UserEmailUpdateCommand
	err     error
}

func (w *recordingUserEmailWriter) UpdateEmail(
	_ context.Context,
	command UserEmailUpdateCommand,
) error {
	w.calls++
	w.command = command
	return w.err
}
