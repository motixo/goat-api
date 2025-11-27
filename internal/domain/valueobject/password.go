package valueobject

type Password struct {
	value string
}

func (p Password) Value() string {
	return p.value
}

func (p Password) IsZero() bool {
	return p.value == ""
}

func PasswordFromHash(hash string) Password {
	return Password{value: hash}
}
