package user

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
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

func userRowFromDomain(user *entity.User) userRow {
	return userRow{
		ID:                user.ID,
		Email:             user.Email,
		PasswordHash:      user.Password.Encoded(),
		Status:            int16(user.Status),
		Role:              int16(user.Role),
		CredentialVersion: user.CredentialVersion,
		CreatedAt:         user.CreatedAt,
		UpdatedAt:         user.UpdatedAt,
	}
}

func (row userRow) toDomain() *entity.User {
	return &entity.User{
		ID:                row.ID,
		Email:             row.Email,
		Password:          valueobject.PasswordFromHash(row.PasswordHash),
		Status:            valueobject.UserStatus(row.Status),
		Role:              valueobject.UserRole(row.Role),
		CredentialVersion: row.CredentialVersion,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func userRowsToDomain(rows []userRow) []*entity.User {
	users := make([]*entity.User, len(rows))
	for index := range rows {
		users[index] = rows[index].toDomain()
	}
	return users
}
