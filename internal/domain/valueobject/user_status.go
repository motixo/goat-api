package valueobject

type UserStatus uint8

const (
	StatusInactive UserStatus = iota
	StatusActive
	StatusSuspended
)

func (s UserStatus) String() string {
	return [...]string{"inactive", "active", "suspended"}[s]
}

func (s UserStatus) IsValid() bool {
	return s <= StatusSuspended
}
