package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func CheckPassword(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

func MakeJWT(userId uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		&jwt.RegisteredClaims{
			Issuer:    "chirpy-access",
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expiresIn)),
			Subject:   userId.String(),
		},
	)

	ret, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	return ret, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claimStruct := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		&claimStruct,
		func(token *jwt.Token) (any, error) {
			return []byte(tokenSecret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return uuid.Nil, err
	}

	if _, ok := token.Claims.(*jwt.RegisteredClaims); !ok {
		return uuid.Nil, errors.New("claims not jwt.RegisteredClaims")
	}

	if !token.Valid {
		return uuid.Nil, errors.New("token is invalid")
	}

	subject, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, err
	}

	issuer, err := token.Claims.GetIssuer()
	if err != nil {
		return uuid.Nil, err
	}
	if issuer != "chirpy-access" {
		return uuid.Nil, errors.New("wrong issuer")
	}

	ret, err := uuid.Parse(subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse uuid: %v", err)
	}
	if ret == uuid.Nil {
		return uuid.Nil, fmt.Errorf("parsed uuid is nil: %v", ret)
	}

	return ret, nil
}
