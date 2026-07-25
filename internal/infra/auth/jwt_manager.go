package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type JWTManager struct {
	secret []byte
	now    func() time.Time
}

type tokenClaims struct {
	UserID            string   `json:"user_id"`
	SessionID         string   `json:"session_id"`
	CredentialVersion int64    `json:"credential_version"`
	TokenType         string   `json:"token_type"`
	Role              string   `json:"role,omitempty"`
	Permissions       []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

func NewJWTManager(secret string) *JWTManager {
	return newJWTManagerWithClock(secret, time.Now)
}

func newJWTManagerWithClock(secret string, now func() time.Time) *JWTManager {
	if now == nil {
		now = time.Now
	}
	return &JWTManager{
		secret: []byte(secret),
		now:    now,
	}
}

func (j *JWTManager) GenerateAccessToken(
	identity valueobject.TokenIdentity,
	snapshot valueobject.AuthorizationSnapshot,
	duration time.Duration,
) (string, *valueobject.JWTClaims, error) {
	return j.generateToken(
		identity,
		valueobject.TokenTypeAccess,
		snapshot,
		duration,
	)
}

func (j *JWTManager) GenerateRefreshToken(
	identity valueobject.TokenIdentity,
	duration time.Duration,
) (string, *valueobject.JWTClaims, error) {
	return j.generateToken(
		identity,
		valueobject.TokenTypeRefresh,
		valueobject.AuthorizationSnapshot{},
		duration,
	)
}

func (j *JWTManager) generateToken(
	identity valueobject.TokenIdentity,
	tokenType valueobject.TokenType,
	snapshot valueobject.AuthorizationSnapshot,
	duration time.Duration,
) (string, *valueobject.JWTClaims, error) {
	now := j.now().UTC()
	claimsVO, err := valueobject.NewJWTClaims(
		identity,
		tokenType,
		now.Add(duration),
		now,
		snapshot,
	)
	if err != nil {
		return "", nil, err
	}

	claims := tokenClaimsFromDomain(claimsVO)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", nil, domainErrors.ErrInternal
	}
	return signed, claimsVO, nil
}

func tokenClaimsFromDomain(claims *valueobject.JWTClaims) tokenClaims {
	permissions := claims.Permissions.Values()
	permissionClaims := make([]string, len(permissions))
	for index := range permissions {
		permissionClaims[index] = permissions[index].String()
	}

	role := ""
	if claims.IsAccess() {
		role = claims.Role.String()
	}
	return tokenClaims{
		UserID:            claims.UserID,
		SessionID:         claims.SessionID,
		CredentialVersion: claims.CredentialVersion,
		TokenType:         string(claims.TokenType),
		Role:              role,
		Permissions:       permissionClaims,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    claims.Issuer,
			Subject:   claims.Subject,
			Audience:  jwt.ClaimStrings(claims.Audience),
			ExpiresAt: jwt.NewNumericDate(claims.ExpiresAt),
			NotBefore: jwt.NewNumericDate(claims.NotBefore),
			IssuedAt:  jwt.NewNumericDate(claims.IssuedAt),
			ID:        claims.JTI,
		},
	}
}

func (j *JWTManager) ParseAndValidate(tokenString string) (*valueobject.JWTClaims, error) {
	claims := &tokenClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			return j.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(valueobject.TokenIssuer),
		jwt.WithAudience(valueobject.TokenAudience),
		jwt.WithTimeFunc(j.now),
	)
	if err != nil {
		return nil, classifyTokenError(err)
	}
	if !token.Valid {
		return nil, domainErrors.ErrTokenInvalid
	}

	claimsVO, err := claims.toDomain()
	if err != nil {
		return nil, domainErrors.ErrTokenInvalid
	}
	if err := j.ValidateClaims(claimsVO); err != nil {
		return nil, err
	}
	return claimsVO, nil
}

func (claims tokenClaims) toDomain() (*valueobject.JWTClaims, error) {
	tokenType := valueobject.TokenType(claims.TokenType)
	if claims.Subject != claims.TokenType ||
		(tokenType != valueobject.TokenTypeAccess &&
			tokenType != valueobject.TokenTypeRefresh) {
		return nil, domainErrors.ErrTokenInvalid
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil || claims.NotBefore == nil {
		return nil, domainErrors.ErrTokenInvalid
	}

	snapshot := valueobject.AuthorizationSnapshot{}
	if tokenType == valueobject.TokenTypeAccess {
		role, err := valueobject.ParseUserRole(claims.Role)
		if err != nil {
			return nil, domainErrors.ErrTokenInvalid
		}
		permissions := make([]valueobject.Permission, len(claims.Permissions))
		for index := range claims.Permissions {
			permission, err := valueobject.ParsePermission(claims.Permissions[index])
			if err != nil {
				return nil, domainErrors.ErrTokenInvalid
			}
			permissions[index] = permission
		}
		permissionSet, err := valueobject.NewPermissionSet(permissions)
		if err != nil {
			return nil, domainErrors.ErrTokenInvalid
		}
		snapshot = valueobject.AuthorizationSnapshot{
			Role:        role,
			Permissions: permissionSet,
		}
	} else if claims.Role != "" || len(claims.Permissions) != 0 {
		return nil, domainErrors.ErrTokenInvalid
	}

	return valueobject.NewJWTClaims(
		valueobject.TokenIdentity{
			UserID:            claims.UserID,
			SessionID:         claims.SessionID,
			JTI:               claims.ID,
			CredentialVersion: claims.CredentialVersion,
		},
		tokenType,
		claims.ExpiresAt.Time,
		claims.IssuedAt.Time,
		snapshot,
	)
}

func classifyTokenError(err error) error {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return domainErrors.ErrTokenExpired
	case errors.Is(err, jwt.ErrTokenMalformed),
		errors.Is(err, jwt.ErrSignatureInvalid),
		errors.Is(err, jwt.ErrTokenNotValidYet):
		return domainErrors.ErrTokenInvalid
	default:
		return domainErrors.ErrTokenInvalid
	}
}

func (j *JWTManager) ValidateClaims(claims *valueobject.JWTClaims) error {
	if claims == nil {
		return domainErrors.ErrTokenInvalid
	}
	now := j.now()
	if now.Before(claims.NotBefore) {
		return domainErrors.ErrTokenInvalid
	}
	if !now.Before(claims.ExpiresAt) {
		return domainErrors.ErrTokenExpired
	}
	if claims.UserID == "" ||
		claims.SessionID == "" ||
		claims.JTI == "" ||
		claims.CredentialVersion <= 0 {
		return domainErrors.ErrTokenInvalid
	}
	if claims.IsAccess() && claims.Role == valueobject.RoleUnknown {
		return domainErrors.ErrTokenInvalid
	}
	if !claims.IsAccess() && !claims.IsRefresh() {
		return domainErrors.ErrTokenInvalid
	}
	return nil
}
