package service

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/motixo/goat-api/internal/domain/errors"
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

	claimsVO, err := valueobject.NewJWTClaims(userID, valueobject.TokenTypeAccess, sessionID, jti, expiresAt)
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
		return "", nil, err
	}

	return signed, claimsVO, nil
}

func (j *JWTManager) GenerateRefreshToken(userID, jti string, duration time.Duration) (string, *valueobject.JWTClaims, error) {
	expiresAt := time.Now().UTC().Add(duration)

	claimsVO, err := valueobject.NewJWTClaims(userID, valueobject.TokenTypeRefresh, "", jti, expiresAt)
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
		return "", nil, err
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
		return nil, err
	}
	if !token.Valid {
		return nil, errors.ErrUnauthorized
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.ErrUnauthorized
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return nil, errors.ErrUnauthorized
	}

	sessionID, _ := claims["session_id"].(string)

	tokenTypeStr, ok := claims["token_type"].(string)
	if !ok {
		return nil, errors.ErrUnauthorized
	}

	jti, ok := claims["jti"].(string)
	if !ok {
		return nil, errors.ErrUnauthorized
	}

	expiresAt := time.Unix(int64(claims["exp"].(float64)), 0)
	issuedAt := time.Unix(int64(claims["iat"].(float64)), 0)

	return &valueobject.JWTClaims{
		UserID:    userID,
		SessionID: sessionID,
		TokenType: valueobject.TokenType(tokenTypeStr),
		JTI:       jti,
		ExpiresAt: expiresAt,
		IssuedAt:  issuedAt,
	}, nil
}

func (j *JWTManager) ValidateClaims(claims *valueobject.JWTClaims) error {
	if claims.IsExpired() {
		return errors.ErrTokenExpired
	}
	if !claims.IsValid() {
		return errors.ErrUnauthorized
	}
	return nil
}
