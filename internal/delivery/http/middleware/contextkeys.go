package middleware

type ContextKey string

const (
	UserIDKey    ContextKey = "user_id"
	SessionIDKey ContextKey = "session_id"
)
