package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	DomainError "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type JWTManager struct {
	secret []byte
}

func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
	}
}

func (j *JWTManager) GenerateAccessToken(userID, sessionID, jti string, duration time.Duration) (string, *valueobject.JWTClaims, error) {
	expiresAt := time.Now().UTC().Add(duration)

	claimsVO, err := valueobject.NewJWTClaims(userID, sessionID, valueobject.TokenTypeAccess, jti, expiresAt)
	if err != nil {
		return "", nil, err
	}

	jwtClaims := jwt.MapClaims{
		"user_id":    claimsVO.UserID,
		"token_type": string(claimsVO.TokenType),
		"session_id": claimsVO.SessionID,
		"jti":        claimsVO.JTI,
		"iss":        claimsVO.Issuer,
		"sub":        claimsVO.Subject,
		"aud":        claimsVO.Audience,
		"exp":        claimsVO.ExpiresAt.Unix(),
		"iat":        claimsVO.IssuedAt.Unix(),
		"nbf":        claimsVO.NotBefore.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", nil, DomainError.ErrInternal
	}

	return signed, claimsVO, nil
}

func (j *JWTManager) GenerateRefreshToken(userID, jti string, duration time.Duration) (string, *valueobject.JWTClaims, error) {
	expiresAt := time.Now().UTC().Add(duration)

	claimsVO, err := valueobject.NewJWTClaims(userID, "", valueobject.TokenTypeRefresh, jti, expiresAt)
	if err != nil {
		return "", nil, err
	}

	jwtClaims := jwt.MapClaims{
		"user_id":    claimsVO.UserID,
		"token_type": string(claimsVO.TokenType),
		"jti":        claimsVO.JTI,
		"iss":        claimsVO.Issuer,
		"sub":        claimsVO.Subject,
		"aud":        claimsVO.Audience,
		"exp":        claimsVO.ExpiresAt.Unix(),
		"iat":        claimsVO.IssuedAt.Unix(),
		"nbf":        claimsVO.NotBefore.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", nil, DomainError.ErrInternal
	}

	return signed, claimsVO, nil
}

func (j *JWTManager) ParseAndValidate(tokenStr string) (*valueobject.JWTClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return j.secret, nil
	}, jwt.WithLeeway(5*time.Second))

	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, DomainError.ErrTokenExpired
		case errors.Is(err, jwt.ErrTokenMalformed),
			errors.Is(err, jwt.ErrSignatureInvalid),
			errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, DomainError.ErrUnauthorized
		default:
			return nil, DomainError.ErrUnauthorized
		}
	}

	if !token.Valid {
		return nil, DomainError.ErrUnauthorized
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, DomainError.ErrUnauthorized
	}

	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return nil, DomainError.ErrUnauthorized
	}

	jti, ok := claims["jti"].(string)
	if !ok || jti == "" {
		return nil, DomainError.ErrUnauthorized
	}

	tokenTypeStr, ok := claims["token_type"].(string)
	if !ok {
		return nil, DomainError.ErrUnauthorized
	}
	tokenType := valueobject.TokenType(tokenTypeStr)

	var audience []string
	if aud, ok := claims["aud"].([]interface{}); ok {
		for _, a := range aud {
			if s, ok := a.(string); ok {
				audience = append(audience, s)
			}
		}
	} else if audStr, ok := claims["aud"].(string); ok {
		audience = []string{audStr}
	} else {
		return nil, DomainError.ErrUnauthorized
	}

	issuer, _ := claims["iss"].(string)
	if issuer != valueobject.TokenIssuer {
		return nil, DomainError.ErrUnauthorized
	}

	hasValidAud := false
	for _, aud := range audience {
		if aud == valueobject.TokenAudience {
			hasValidAud = true
			break
		}
	}
	if !hasValidAud {
		return nil, DomainError.ErrUnauthorized
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, DomainError.ErrUnauthorized
	}
	iat, ok := claims["iat"].(float64)
	if !ok {
		return nil, DomainError.ErrUnauthorized
	}
	nbf, ok := claims["nbf"].(float64)
	if !ok {
		return nil, DomainError.ErrUnauthorized
	}

	sessionID, _ := claims["session_id"].(string)

	return &valueobject.JWTClaims{
		UserID:    userID,
		SessionID: sessionID,
		TokenType: tokenType,
		JTI:       jti,
		Issuer:    issuer,
		Subject:   tokenTypeStr,
		Audience:  audience,
		ExpiresAt: time.Unix(int64(exp), 0),
		IssuedAt:  time.Unix(int64(iat), 0),
		NotBefore: time.Unix(int64(nbf), 0),
	}, nil
}

func (j *JWTManager) ValidateClaims(claims *valueobject.JWTClaims) error {
	now := time.Now()

	if now.Before(claims.NotBefore) {
		return DomainError.ErrUnauthorized
	}

	if now.After(claims.ExpiresAt) {
		return DomainError.ErrTokenExpired
	}

	if claims.UserID == "" || claims.JTI == "" {
		return DomainError.ErrUnauthorized
	}

	if claims.IsAccess() {
		if claims.SessionID == "" {
			return DomainError.ErrUnauthorized
		}
	}

	return nil
}
