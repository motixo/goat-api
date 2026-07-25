package session

import (
	"context"
	stdErrors "errors"
	"reflect"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
)

const (
	currentSessionID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	secondSessionID  = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	foreignSessionID = "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	missingSessionID = "01ARZ3NDEKTSV4RRFFQ69G5FAY"
	concurrentID     = "01ARZ3NDEKTSV4RRFFQ69G5FAZ"
)

func TestDeleteSessionsAllowsSessionsOwnedByCurrentUser(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
	}{
		{name: "current session", targets: []string{currentSessionID}},
		{name: "another session", targets: []string{secondSessionID}},
		{name: "multiple sessions", targets: []string{currentSessionID, secondSessionID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newOwnershipSessionRepository(map[string]string{
				currentSessionID: "user-1",
				secondSessionID:  "user-1",
			})
			usecase := NewUsecase(repo, discardSessionLogger{})

			err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
				UserID:         "user-1",
				CurrentSession: currentSessionID,
				TargetSessions: tt.targets,
			})

			if err != nil {
				t.Fatalf("DeleteSessions() error = %v", err)
			}
			if !reflect.DeepEqual(repo.deletedIDs, tt.targets) {
				t.Fatalf("deleted session IDs = %v, want %v", repo.deletedIDs, tt.targets)
			}
			if repo.directDeleteCalls != 0 {
				t.Fatalf("unscoped Delete() calls = %d, want 0", repo.directDeleteCalls)
			}
		})
	}
}

func TestDeleteSessionsRejectsSessionOwnedByAnotherUser(t *testing.T) {
	repo := newOwnershipSessionRepository(map[string]string{
		foreignSessionID: "user-2",
	})
	usecase := NewUsecase(repo, discardSessionLogger{})

	err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
		UserID:         "user-1",
		TargetSessions: []string{foreignSessionID},
	})

	if !stdErrors.Is(err, domainErrors.ErrNotFound) {
		t.Fatalf("DeleteSessions() error = %v, want ErrNotFound", err)
	}
	if len(repo.deletedIDs) != 0 {
		t.Fatalf("deleted session IDs = %v, want no mutation", repo.deletedIDs)
	}
	if owner := repo.owners[foreignSessionID]; owner != "user-2" {
		t.Fatalf("foreign session owner = %q, want %q", owner, "user-2")
	}
	if repo.directDeleteCalls != 0 {
		t.Fatalf("unscoped Delete() calls = %d, want 0", repo.directDeleteCalls)
	}
}

func TestDeleteSessionsRejectsMixedOwnershipWithoutMutation(t *testing.T) {
	repo := newOwnershipSessionRepository(map[string]string{
		currentSessionID: "user-1",
		foreignSessionID: "user-2",
	})
	usecase := NewUsecase(repo, discardSessionLogger{})

	err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
		UserID:         "user-1",
		TargetSessions: []string{currentSessionID, foreignSessionID},
	})

	if !stdErrors.Is(err, domainErrors.ErrNotFound) {
		t.Fatalf("DeleteSessions() error = %v, want ErrNotFound", err)
	}
	if len(repo.deletedIDs) != 0 {
		t.Fatalf("deleted session IDs = %v, want no mutation", repo.deletedIDs)
	}
	wantOwners := map[string]string{currentSessionID: "user-1", foreignSessionID: "user-2"}
	if !reflect.DeepEqual(repo.owners, wantOwners) {
		t.Fatalf("session owners = %v, want %v", repo.owners, wantOwners)
	}
}

func TestDeleteSessionsMissingSessionUsesSameNotFoundResult(t *testing.T) {
	repo := newOwnershipSessionRepository(nil)
	usecase := NewUsecase(repo, discardSessionLogger{})

	err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
		UserID:         "user-1",
		TargetSessions: []string{missingSessionID},
	})

	if !stdErrors.Is(err, domainErrors.ErrNotFound) {
		t.Fatalf("DeleteSessions() error = %v, want ErrNotFound", err)
	}
	if len(repo.deletedIDs) != 0 {
		t.Fatalf("deleted session IDs = %v, want no mutation", repo.deletedIDs)
	}
}

func TestDeleteSessionsRejectsMalformedSessionIDBeforeRedis(t *testing.T) {
	repo := newOwnershipSessionRepository(nil)
	usecase := NewUsecase(repo, discardSessionLogger{})

	err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
		UserID:         "user-1",
		TargetSessions: []string{"not-a-ulid"},
	})

	if !stdErrors.Is(err, ErrInvalidSessionSelection) {
		t.Fatalf("DeleteSessions() error = %v, want ErrInvalidSessionSelection", err)
	}
	if repo.deleteByUserCalls != 0 {
		t.Fatalf("DeleteByUser() calls = %d, want 0", repo.deleteByUserCalls)
	}
}

func TestDeleteSessionsRemoveOthersScopesTargetsToCurrentUser(t *testing.T) {
	repo := newOwnershipSessionRepository(map[string]string{
		currentSessionID: "user-1",
		secondSessionID:  "user-1",
		foreignSessionID: "user-2",
	})
	repo.listedSessions = []*entity.Session{
		{ID: currentSessionID, UserID: "user-1"},
		{ID: secondSessionID, UserID: "user-1"},
	}
	usecase := NewUsecase(repo, discardSessionLogger{})

	err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
		UserID:         "user-1",
		CurrentSession: currentSessionID,
		TargetSessions: []string{foreignSessionID},
		RemoveOthers:   true,
	})

	if err != nil {
		t.Fatalf("DeleteSessions() error = %v", err)
	}
	if !reflect.DeepEqual(repo.deletedIDs, []string{secondSessionID}) {
		t.Fatalf("deleted session IDs = %v, want [%s]", repo.deletedIDs, secondSessionID)
	}
	if repo.owners[currentSessionID] != "user-1" {
		t.Fatal("current session was deleted")
	}
	if repo.owners[foreignSessionID] != "user-2" {
		t.Fatal("foreign session was deleted")
	}
	if repo.listByUserCalls != 0 {
		t.Fatalf("ListByUser() calls = %d, want 0", repo.listByUserCalls)
	}
	if repo.deleteByUserCalls != 0 {
		t.Fatalf("DeleteByUser() calls = %d, want 0", repo.deleteByUserCalls)
	}
	if repo.deleteOthersCalls != 1 {
		t.Fatalf("DeleteOthersByUser() calls = %d, want 1", repo.deleteOthersCalls)
	}
}

func TestDeleteSessionsRemoveOthersIncludesSessionMissingFromEarlierSnapshot(t *testing.T) {
	repo := newOwnershipSessionRepository(map[string]string{
		currentSessionID: "user-1",
		secondSessionID:  "user-1",
		concurrentID:     "user-1",
	})
	repo.listedSessions = []*entity.Session{
		{ID: currentSessionID, UserID: "user-1"},
		{ID: secondSessionID, UserID: "user-1"},
	}
	usecase := NewUsecase(repo, discardSessionLogger{})

	err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
		UserID:         "user-1",
		CurrentSession: currentSessionID,
		RemoveOthers:   true,
	})

	if err != nil {
		t.Fatalf("DeleteSessions() error = %v", err)
	}
	wantOwners := map[string]string{currentSessionID: "user-1"}
	if !reflect.DeepEqual(repo.owners, wantOwners) {
		t.Fatalf("remaining sessions = %v, want only current session", repo.owners)
	}
}

func TestDeleteSessionsRemoveOthersIsIdempotentWhenNoOthersExist(t *testing.T) {
	repo := newOwnershipSessionRepository(map[string]string{currentSessionID: "user-1"})
	usecase := NewUsecase(repo, discardSessionLogger{})
	input := DeleteSessionsInput{
		UserID:         "user-1",
		CurrentSession: currentSessionID,
		RemoveOthers:   true,
	}

	for attempt := 0; attempt < 2; attempt++ {
		if err := usecase.DeleteSessions(context.Background(), input); err != nil {
			t.Fatalf("DeleteSessions() attempt %d error = %v", attempt+1, err)
		}
	}
	if repo.owners[currentSessionID] != "user-1" {
		t.Fatal("current session was deleted")
	}
	if repo.deleteOthersCalls != 2 {
		t.Fatalf("DeleteOthersByUser() calls = %d, want 2", repo.deleteOthersCalls)
	}
}

func TestDeleteSessionsRemoveOthersRejectsMalformedCurrentSessionBeforeRedis(t *testing.T) {
	repo := newOwnershipSessionRepository(map[string]string{secondSessionID: "user-1"})
	usecase := NewUsecase(repo, discardSessionLogger{})

	err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
		UserID:         "user-1",
		CurrentSession: "not-a-ulid",
		RemoveOthers:   true,
	})

	if !stdErrors.Is(err, ErrInvalidSessionSelection) {
		t.Fatalf("DeleteSessions() error = %v, want ErrInvalidSessionSelection", err)
	}
	if repo.deleteOthersCalls != 0 {
		t.Fatalf("DeleteOthersByUser() calls = %d, want 0", repo.deleteOthersCalls)
	}
	if repo.owners[secondSessionID] != "user-1" {
		t.Fatal("session was mutated after validation failure")
	}
}

func TestDeleteSessionsRemoveOthersHidesInvalidCurrentOwnershipWithoutMutation(t *testing.T) {
	tests := []struct {
		name   string
		owners map[string]string
	}{
		{name: "missing current session", owners: map[string]string{secondSessionID: "user-1"}},
		{name: "foreign current session", owners: map[string]string{
			currentSessionID: "user-2",
			secondSessionID:  "user-1",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newOwnershipSessionRepository(tt.owners)
			before := make(map[string]string, len(repo.owners))
			for sessionID, userID := range repo.owners {
				before[sessionID] = userID
			}
			usecase := NewUsecase(repo, discardSessionLogger{})

			err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
				UserID:         "user-1",
				CurrentSession: currentSessionID,
				RemoveOthers:   true,
			})

			if !stdErrors.Is(err, domainErrors.ErrNotFound) {
				t.Fatalf("DeleteSessions() error = %v, want ErrNotFound", err)
			}
			if !reflect.DeepEqual(repo.owners, before) {
				t.Fatalf("sessions after failed ownership = %v, want %v", repo.owners, before)
			}
		})
	}
}

func TestDeleteSessionsRemoveOthersRepositoryFailureIsReturned(t *testing.T) {
	repositoryErr := stdErrors.New("redis unavailable")
	repo := newOwnershipSessionRepository(map[string]string{currentSessionID: "user-1"})
	repo.deleteOthersErr = repositoryErr
	usecase := NewUsecase(repo, discardSessionLogger{})

	err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
		UserID:         "user-1",
		CurrentSession: currentSessionID,
		RemoveOthers:   true,
	})

	if !stdErrors.Is(err, repositoryErr) {
		t.Fatalf("DeleteSessions() error = %v, want repository error", err)
	}
	if repo.owners[currentSessionID] != "user-1" {
		t.Fatal("current session was mutated after repository failure")
	}
}

func TestDeleteSessionsRepositoryFailureIsReturned(t *testing.T) {
	repositoryErr := stdErrors.New("redis unavailable")
	repo := newOwnershipSessionRepository(map[string]string{currentSessionID: "user-1"})
	repo.deleteByUserErr = repositoryErr
	usecase := NewUsecase(repo, discardSessionLogger{})

	err := usecase.DeleteSessions(context.Background(), DeleteSessionsInput{
		UserID:         "user-1",
		TargetSessions: []string{currentSessionID},
	})

	if !stdErrors.Is(err, repositoryErr) {
		t.Fatalf("DeleteSessions() error = %v, want repository error", err)
	}
	if len(repo.deletedIDs) != 0 {
		t.Fatalf("deleted session IDs = %v, want no recorded mutation", repo.deletedIDs)
	}
}

type ownershipSessionRepository struct {
	repository.SessionRepository
	owners            map[string]string
	listedSessions    []*entity.Session
	deletedIDs        []string
	deleteByUserErr   error
	deleteOthersErr   error
	deleteByUserCalls int
	deleteOthersCalls int
	listByUserCalls   int
	directDeleteCalls int
}

func newOwnershipSessionRepository(owners map[string]string) *ownershipSessionRepository {
	copyOwners := make(map[string]string, len(owners))
	for sessionID, userID := range owners {
		copyOwners[sessionID] = userID
	}
	return &ownershipSessionRepository{owners: copyOwners}
}

func (r *ownershipSessionRepository) ListByUser(context.Context, string, int, int) ([]*entity.Session, int64, error) {
	r.listByUserCalls++
	return r.listedSessions, int64(len(r.listedSessions)), nil
}

func (r *ownershipSessionRepository) Delete(_ context.Context, sessionIDs []string) error {
	r.directDeleteCalls++
	r.deletedIDs = append(r.deletedIDs, sessionIDs...)
	return nil
}

func (r *ownershipSessionRepository) DeleteByUser(_ context.Context, userID string, sessionIDs []string) (bool, error) {
	r.deleteByUserCalls++
	if r.deleteByUserErr != nil {
		return false, r.deleteByUserErr
	}
	for _, sessionID := range sessionIDs {
		owner, exists := r.owners[sessionID]
		if !exists || owner != userID {
			return false, nil
		}
	}
	for _, sessionID := range sessionIDs {
		delete(r.owners, sessionID)
		r.deletedIDs = append(r.deletedIDs, sessionID)
	}
	return true, nil
}

func (r *ownershipSessionRepository) DeleteOthersByUser(_ context.Context, userID, currentSessionID string) (bool, error) {
	r.deleteOthersCalls++
	if r.deleteOthersErr != nil {
		return false, r.deleteOthersErr
	}
	owner, exists := r.owners[currentSessionID]
	if !exists || owner != userID {
		return false, nil
	}
	for sessionID, sessionOwner := range r.owners {
		if sessionOwner == userID && sessionID != currentSessionID {
			delete(r.owners, sessionID)
			r.deletedIDs = append(r.deletedIDs, sessionID)
		}
	}
	return true, nil
}

type discardSessionLogger struct{}

func (discardSessionLogger) Info(string, ...any)  {}
func (discardSessionLogger) Error(string, ...any) {}
func (discardSessionLogger) Warn(string, ...any)  {}
func (discardSessionLogger) Debug(string, ...any) {}
func (discardSessionLogger) Panic(string, ...any) {}
