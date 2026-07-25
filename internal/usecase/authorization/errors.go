package authorization

import "errors"

var (
	// ErrPrincipalInactive means the authoritative user exists but is not yet
	// eligible to authenticate.
	ErrPrincipalInactive = errors.New("principal is inactive")
	// ErrPrincipalSuspended means the authoritative user is suspended.
	ErrPrincipalSuspended = errors.New("principal is suspended")
	// ErrPrincipalSecurityStateChanged means a verified token/session snapshot
	// no longer matches authoritative user state.
	ErrPrincipalSecurityStateChanged = errors.New("principal security state changed")
	// ErrPermissionDenied means the current authoritative or signed snapshot
	// does not grant the requested action.
	ErrPermissionDenied = errors.New("permission denied")
)
