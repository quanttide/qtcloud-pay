package security

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
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
	secretKey []byte
	authKeys  []*rsa.PublicKey
	issuer    string
	audience  string
	now       func() time.Time
}

func NewTokenVerifier(secretKey, authPublicJWK, authPublicPEM, issuer, audience string) (*TokenVerifier, error) {
	secretKey = strings.TrimSpace(secretKey)
	if secretKey == "" {
		return nil, ErrMissingSecretKey
	}
	v := &TokenVerifier{
		secretKey: []byte(secretKey),
		issuer:    strings.TrimSpace(issuer),
		audience:  strings.TrimSpace(audience),
		now:       time.Now,
	}
	if strings.TrimSpace(authPublicJWK) != "" {
		keys, err := parseJWKSet(authPublicJWK)
		if err != nil {
			return nil, err
		}
		v.authKeys = append(v.authKeys, keys...)
	}
	if strings.TrimSpace(authPublicPEM) != "" {
		key, err := parseRSAPublicPEM(authPublicPEM)
		if err != nil {
			return nil, err
		}
		v.authKeys = append(v.authKeys, key)
	}
	return v, nil
}

func (v *TokenVerifier) SignServiceToken(subject string, ttl time.Duration) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" || ttl <= 0 {
		return "", ErrInvalidToken
	}
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	payload := map[string]any{
		"iss": "qtcloud-pay",
		"sub": subject,
		"aud": "qtcloud-pay",
		"exp": v.now().Add(ttl).Unix(),
		"iat": v.now().Unix(),
		"jti": randomID(),
	}
	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	unsigned := enc(hb) + "." + enc(pb)
	return unsigned + "." + signHS256(unsigned, v.secretKey), nil
}

func (v *TokenVerifier) Verify(raw string) (*Claims, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	headerBytes, err := dec(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ErrInvalidToken
	}
	unsigned := parts[0] + "." + parts[1]
	sig, err := dec(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}
	switch header.Alg {
	case "HS256":
		want, _ := dec(signHS256(unsigned, v.secretKey))
		if !hmac.Equal(sig, want) {
			return nil, ErrInvalidToken
		}
	case "RS256":
		if len(v.authKeys) == 0 {
			return nil, ErrInvalidToken
		}
		sum := sha256.Sum256([]byte(unsigned))
		ok := false
		for _, key := range v.authKeys {
			if rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) == nil {
				ok = true
				break
			}
		}
		if !ok {
			return nil, ErrInvalidToken
		}
	default:
		return nil, ErrInvalidToken
	}
	claims, err := parseClaims(parts[1])
	if err != nil {
		return nil, err
	}
	if !claims.Expiry.IsZero() && !claims.Expiry.After(v.now()) {
		return nil, ErrExpiredToken
	}
	if v.issuer != "" && claims.Issuer != v.issuer {
		return nil, ErrInvalidToken
	}
	if v.audience != "" && claims.Audience != v.audience {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func parseClaims(payloadPart string) (*Claims, error) {
	payloadBytes, err := dec(payloadPart)
	if err != nil {
		return nil, ErrInvalidToken
	}
	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, ErrInvalidToken
	}
	sub := firstString(payload, "sub", "user_id", "customer_id")
	if sub == "" {
		return nil, ErrInvalidToken
	}
	c := &Claims{Subject: sub, Issuer: stringClaim(payload["iss"]), Audience: audienceClaim(payload["aud"]), Roles: stringSliceClaim(payload["roles"])}
	exp, ok := numberClaim(payload["exp"])
	if !ok {
		return nil, ErrInvalidToken
	}
	c.Expiry = time.Unix(int64(exp), 0)
	return c, nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := stringClaim(m[key]); s != "" {
			return s
		}
	}
	return ""
}

func stringClaim(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func audienceClaim(v any) string {
	if s := stringClaim(v); s != "" {
		return s
	}
	if xs, ok := v.([]any); ok && len(xs) > 0 {
		return stringClaim(xs[0])
	}
	return ""
}

func stringSliceClaim(v any) []string {
	xs, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if s := stringClaim(x); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func numberClaim(v any) (float64, bool) {
	n, ok := v.(float64)
	return n, ok
}

func enc(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func dec(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func signHS256(unsigned string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(unsigned))
	return enc(mac.Sum(nil))
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return enc(b)
}

func parseRSAPublicPEM(raw string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, ErrInvalidToken
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, ErrInvalidToken
	}
	return key, nil
}

func parseJWKSet(raw string) ([]*rsa.PublicKey, error) {
	type jwk struct {
		Kty string `json:"kty"`
		N   string `json:"n"`
		E   string `json:"e"`
	}
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal([]byte(raw), &set); err != nil || len(set.Keys) == 0 {
		var one jwk
		if err2 := json.Unmarshal([]byte(raw), &one); err2 != nil {
			if err != nil {
				return nil, err
			}
			return nil, err2
		}
		if one.Kty == "" && one.N == "" && one.E == "" {
			if err != nil {
				return nil, err
			}
			return nil, ErrInvalidToken
		}
		set.Keys = append(set.Keys, one)
	}
	keys := make([]*rsa.PublicKey, 0, len(set.Keys))
	for _, item := range set.Keys {
		if item.Kty != "RSA" {
			continue
		}
		nb, err := dec(item.N)
		if err != nil {
			return nil, err
		}
		eb, err := dec(item.E)
		if err != nil {
			return nil, err
		}
		e := 0
		for _, b := range eb {
			e = e<<8 + int(b)
		}
		keys = append(keys, &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e})
	}
	if len(keys) == 0 {
		return nil, ErrInvalidToken
	}
	return keys, nil
}
