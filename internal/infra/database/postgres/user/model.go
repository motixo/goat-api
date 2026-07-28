package user

import (
	"fmt"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	userdetail "github.com/motixo/goat-api/internal/usecase/user/detail"
	userlisting "github.com/motixo/goat-api/internal/usecase/user/listing"
	userstatuschange "github.com/motixo/goat-api/internal/usecase/user/statuschange"
)

type userRow struct {
	ID                string     `db:"id"`
	Email             string     `db:"email"`
	PasswordHash      string     `db:"password"`
	Status            int16      `db:"status"`
	Role              int16      `db:"role"`
	CredentialVersion int64      `db:"credential_version"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         *time.Time `db:"updated_at"`
}

type userListRow struct {
	ID        string    `db:"id"`
	Email     string    `db:"email"`
	Status    int16     `db:"status"`
	Role      int16     `db:"role"`
	CreatedAt time.Time `db:"created_at"`
}

type userDetailRow struct {
	ID        string    `db:"id"`
	Email     string    `db:"email"`
	Status    int16     `db:"status"`
	Role      int16     `db:"role"`
	CreatedAt time.Time `db:"created_at"`
}

type userStatusSnapshotRow struct {
	ID     string `db:"id"`
	Role   int16  `db:"role"`
	Status int16  `db:"status"`
}

func userRowFromDomain(user *entity.User) userRow {
	return userRow{
		ID:                user.ID,
		Email:             user.Email,
		PasswordHash:      user.PasswordDigest.Encoded(),
		Status:            int16(user.Status),
		Role:              int16(user.Role),
		CredentialVersion: user.CredentialVersion,
		CreatedAt:         user.CreatedAt,
		UpdatedAt:         user.UpdatedAt,
	}
}

func (row userRow) toDomain() (*entity.User, error) {
	digest, err := valueobject.NewPasswordDigest(row.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("rehydrate user password digest: %w", err)
	}
	return &entity.User{
		ID:                row.ID,
		Email:             row.Email,
		PasswordDigest:    digest,
		Status:            valueobject.UserStatus(row.Status),
		Role:              valueobject.UserRole(row.Role),
		CredentialVersion: row.CredentialVersion,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

func (row userListRow) toListItem() userlisting.UserListItem {
	return userlisting.UserListItem{
		ID:        row.ID,
		Email:     row.Email,
		Status:    valueobject.UserStatus(row.Status),
		Role:      valueobject.UserRole(row.Role),
		CreatedAt: row.CreatedAt,
	}
}

func (row userDetailRow) toDetail() userdetail.UserDetail {
	return userdetail.UserDetail{
		ID:        row.ID,
		Email:     row.Email,
		Status:    valueobject.UserStatus(row.Status),
		Role:      valueobject.UserRole(row.Role),
		CreatedAt: row.CreatedAt,
	}
}

func (row userStatusSnapshotRow) toStatusSnapshot() userstatuschange.UserStatusSnapshot {
	return userstatuschange.UserStatusSnapshot{
		ID:     row.ID,
		Role:   valueobject.UserRole(row.Role),
		Status: valueobject.UserStatus(row.Status),
	}
}

func userListRowsToItems(rows []userListRow) []userlisting.UserListItem {
	items := make([]userlisting.UserListItem, len(rows))
	for index := range rows {
		items[index] = rows[index].toListItem()
	}
	return items
}
