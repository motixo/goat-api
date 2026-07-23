package auth

// CurrentSessionInvalidError means the server-owned session for the
// authenticated workflow is missing or no longer valid for that principal.
type CurrentSessionInvalidError struct {
	cause error
}

func NewCurrentSessionInvalidError(cause error) *CurrentSessionInvalidError {
	return &CurrentSessionInvalidError{cause: cause}
}

func (e *CurrentSessionInvalidError) Error() string {
	if e == nil || e.cause == nil {
		return "current session is invalid"
	}
	return "current session is invalid: " + e.cause.Error()
}

func (e *CurrentSessionInvalidError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}
