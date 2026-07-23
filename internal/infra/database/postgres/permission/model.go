package permission

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type permissionRow struct {
	ID        string    `db:"id"`
	Role      int16     `db:"role"`
	Action    string    `db:"action"`
	CreatedAt time.Time `db:"created_at"`
}

func permissionRowFromDomain(permission *entity.Permission) permissionRow {
	return permissionRow{
		ID:        permission.ID,
		Role:      int16(permission.Role),
		Action:    permission.Action.String(),
		CreatedAt: permission.CreatedAt,
	}
}

func (row permissionRow) toDomain() *entity.Permission {
	return &entity.Permission{
		ID:        row.ID,
		Role:      valueobject.UserRole(row.Role),
		Action:    valueobject.Permission(row.Action),
		CreatedAt: row.CreatedAt,
	}
}

func permissionRowsToDomain(rows []permissionRow) []*entity.Permission {
	permissions := make([]*entity.Permission, len(rows))
	for index := range rows {
		permissions[index] = rows[index].toDomain()
	}
	return permissions
}
