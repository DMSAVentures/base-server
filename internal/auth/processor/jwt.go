package processor

import (
	"base-server/internal/store"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func (p *AuthProcessor) generateJWTToken(ctx context.Context, user store.AuthenticatedUser) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour) // Token valid for 24 hours
	cl := jwt.New(jwt.SigningMethodHS256)
	claims := cl.Claims.(jwt.MapClaims)
	claims["sub"] = user.UserID.String()
	claims["account_id"] = user.AccountID.String()
	claims["auth_type"] = user.AuthType
	claims["iss"] = "base-server"
	claims["aud"] = "base-server"
	claims["exp"] = expirationTime.Unix()
	claims["iat"] = time.Now().Unix()

	// Create a new token object
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(p.authConfig.Email.JWTSecret))
	if err != nil {
		p.logger.Error(ctx, "failed to sign token", err)
		return "", ErrFailedSignIn
	}

	return tokenString, nil
}

func (b *BaseClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return b.ExpirationTime, nil
}

func (b *BaseClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return b.IssuedAt, nil
}

func (b *BaseClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return b.NotBefore, nil
}

func (b *BaseClaims) GetIssuer() (string, error) {
	return b.Issuer, nil
}

func (b *BaseClaims) GetSubject() (string, error) {
	return b.Subject, nil
}

func (b *BaseClaims) GetAudience() (jwt.ClaimStrings, error) {
	return b.Audience, nil
}

func (p *AuthProcessor) ValidateJWTToken(ctx context.Context, token string) (BaseClaims, error) {
	var baseClaims BaseClaims
	// Parse the token
	t, err := jwt.ParseWithClaims(token, &baseClaims, func(token *jwt.Token) (interface{}, error) {
		// Check the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(p.authConfig.Email.JWTSecret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			p.logger.Error(ctx, "token expired", err)
			return BaseClaims{}, ErrExpiredToken
		}

		p.logger.Error(ctx, "failed to parse token", err)
		return BaseClaims{}, ErrParseJWTToken
	}
	if !t.Valid {
		return BaseClaims{}, ErrInvalidJWTToken
	}

	// Extract claims from the parsed token
	claims, ok := t.Claims.(*BaseClaims)
	if !ok {
		p.logger.Error(ctx, "failed to extract claims", err)
		return BaseClaims{}, ErrParseJWTToken
	}

	return *claims, nil
}
