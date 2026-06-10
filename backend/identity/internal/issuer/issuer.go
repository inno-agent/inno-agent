package issuer

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuerName = "inno-agent-identity"

type Claims struct {
	UserID     string
	Tier       string
	CtxVersion int32
}

type Issuer struct {
	privateKey *rsa.PrivateKey
	expiry     time.Duration
}

func New(privateKeyPEM []byte, expiry time.Duration) (*Issuer, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}

	return &Issuer{privateKey: rsaKey, expiry: expiry}, nil
}

func (i *Issuer) Issue(userID, tier string, ctxVersion int32) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":         userID,
		"tier":        tier,
		"ctx_version": ctxVersion,
		"iss":         issuerName,
		"iat":         jwt.NewNumericDate(now),
		"exp":         jwt.NewNumericDate(now.Add(i.expiry)),
	})
	signed, err := token.SignedString(i.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func (i *Issuer) PublicKeyJWKS() map[string]interface{} {
	pub := &i.privateKey.PublicKey
	return map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			},
		},
	}
}

func (i *Issuer) Verify(tokenStr string) (Claims, error) {
	token, err := jwt.Parse(
		tokenStr,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return &i.privateKey.PublicKey, nil
		},
		jwt.WithIssuer(issuerName),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("verify token: %w", err)
	}

	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return Claims{}, errors.New("invalid claims")
	}

	sub, _ := mc["sub"].(string)
	tier, _ := mc["tier"].(string)
	ctxVersionFloat, _ := mc["ctx_version"].(float64)

	return Claims{
		UserID:     sub,
		Tier:       tier,
		CtxVersion: int32(ctxVersionFloat),
	}, nil
}
