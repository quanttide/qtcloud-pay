package security

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/quanttide/quanttide-auth-toolkit/packages/go/pkg/servicetoken"
	"github.com/quanttide/quanttide-auth-toolkit/packages/go/pkg/tokenverify"
)

var (
	ErrMissingSecretKey = errors.New("SECRET_KEY is required")
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("expired token")
)

type Claims struct {
	Subject  string
	Issuer   string
	Audience string
	Expiry   time.Time
	Roles    []string
}

type TokenVerifier struct {
	verifier *tokenverify.Verifier
	signer   *servicetoken.Signer
}

func NewTokenVerifier(secretKey, authPublicJWK, authPublicPEM, issuer, audience string) (*TokenVerifier, error) {
	secretKey = strings.TrimSpace(secretKey)
	if secretKey == "" {
		return nil, ErrMissingSecretKey
	}
	verifier, err := tokenverify.New(tokenverify.Config{
		HMACSecret:      secretKey,
		StaticPublicJWK: authPublicJWK,
		StaticPublicPEM: authPublicPEM,
		Issuer:          issuer,
		Audience:        audience,
	})
	if err != nil {
		return nil, err
	}
	return &TokenVerifier{
		verifier: verifier,
		signer:   servicetoken.NewSigner(secretKey, "qtcloud-pay", "qtcloud-pay"),
	}, nil
}

func (v *TokenVerifier) SignServiceToken(subject string, ttl time.Duration) (string, error) {
	token, err := v.signer.Sign(subject, ttl)
	if err != nil {
		return "", ErrInvalidToken
	}
	return token, nil
}

func (v *TokenVerifier) Verify(raw string) (*Claims, error) {
	claims, err := v.verifier.Verify(context.Background(), raw)
	if err != nil {
		if errors.Is(err, tokenverify.ErrExpiredToken) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	audience := ""
	if len(claims.Audience) > 0 {
		audience = claims.Audience[0]
	}
	return &Claims{
		Subject:  claims.Subject,
		Issuer:   claims.Issuer,
		Audience: audience,
		Expiry:   claims.ExpiresAt,
		Roles:    claims.Roles,
	}, nil
}

// VerifyWithContext 暴露给新接入代码使用，旧调用方仍可使用 Verify。
func (v *TokenVerifier) VerifyWithContext(ctx context.Context, raw string) (*Claims, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	claims, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		if errors.Is(err, tokenverify.ErrExpiredToken) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	audience := ""
	if len(claims.Audience) > 0 {
		audience = claims.Audience[0]
	}
	return &Claims{Subject: claims.Subject, Issuer: claims.Issuer, Audience: audience, Expiry: claims.ExpiresAt, Roles: claims.Roles}, nil
}
