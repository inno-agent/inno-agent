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
	UserID string
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

	rsaKey, err := parseRSAPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return &Issuer{privateKey: rsaKey, expiry: expiry}, nil
}

// parseRSAPrivateKey accepts both PKCS#8 (-----BEGIN PRIVATE KEY-----) and
// PKCS#1 (-----BEGIN RSA PRIVATE KEY-----) encodings; openssl emits one or the
// other depending on its version and the command used.
func parseRSAPrivateKey(der []byte) (*rsa.PrivateKey, error) {
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
		return rsaKey, nil
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse private key (PKCS#8/PKCS#1): %w", err)
	}
	return rsaKey, nil
}

func (i *Issuer) Issue(userID string) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": userID,
		"iss": issuerName,
		"iat": jwt.NewNumericDate(now),
		"exp": jwt.NewNumericDate(now.Add(i.expiry)),
	})
	signed, err := token.SignedString(i.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func (i *Issuer) IssueService(clientID string) (string, error) {
	return i.Issue("svc:" + clientID)
}

// IssueDelegate issues a short-lived token on behalf of a user, delegated by
// an actor service. sub=userID, act.sub=actorSub, expiry controlled by caller.
func (i *Issuer) IssueDelegate(userID, actorSub string, expiry time.Duration) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": userID,
		"iss": issuerName,
		"iat": jwt.NewNumericDate(now),
		"exp": jwt.NewNumericDate(now.Add(expiry)),
		"act": map[string]string{"sub": actorSub},
	})
	signed, err := token.SignedString(i.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign delegate token: %w", err)
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

	return Claims{UserID: sub}, nil
}
