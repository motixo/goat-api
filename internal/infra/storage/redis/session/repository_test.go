package session

import (
	"context"
	"testing"
)

func TestListByUserRejectsInvalidScopeAndPagination(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		offset int
		limit  int
	}{
		{name: "missing user", offset: 0, limit: 10},
		{name: "negative offset", userID: "user-1", offset: -1, limit: 10},
		{name: "negative limit", userID: "user-1", offset: 0, limit: -1},
	}

	repository := &Repository{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := repository.ListByUser(context.Background(), test.userID, test.offset, test.limit); err == nil {
				t.Fatal("ListByUser accepted invalid input")
			}
		})
	}
}

func TestDeleteAllByUserRejectsEmptyUserIDBeforeRedis(t *testing.T) {
	repository := &Repository{}

	if err := repository.DeleteAllByUser(context.Background(), ""); err == nil {
		t.Fatal("DeleteAllByUser() accepted an empty user ID")
	}
}
