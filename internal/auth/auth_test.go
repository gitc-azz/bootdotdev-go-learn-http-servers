package auth

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashCheckPassword(t *testing.T) {
	tcs := map[string]struct {
		input, another string
	}{
		"one_letter":  {input: "g", another: "h"},
		"four_letter": {input: "fgfe", another: "aeke"},
		"hard":        {input: "zkjzhrguiofd", another: "lkhjafulaiuha"},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			hash, err := HashPassword(tc.input)
			if err != nil {
				t.Error(err)
			}

			if hash == tc.input {
				t.Errorf("password and hash are equal ! -> %v vs %v", tc.input, hash)
			}

			same, err := CheckPassword(tc.input, hash)
			if err != nil {
				t.Error(err)
			}
			if !same {
				t.Error("Produced hash do not match password")
			}

			another_hash, err := HashPassword(tc.another)
			if err != nil {
				t.Error(err)
			}
			not_same, err := CheckPassword(tc.input, another_hash)
			if err != nil {
				t.Error(err)
			}
			if not_same {
				t.Error("another hash matched password")
			}
		})
	}
}

func TestMakeJWT(t *testing.T) {
	tcs := map[string]struct {
		userId      uuid.UUID
		tokenSecret string
		expiresIn   time.Duration
	}{
		"first": {
			userId:      uuid.New(),
			tokenSecret: "123456",
			expiresIn:   time.Duration(2) * time.Second,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			tokenString, err := MakeJWT(tc.userId, tc.tokenSecret, tc.expiresIn)
			if err != nil {
				t.Error(err)
			}

			actualId, err := ValidateJWT(tokenString, tc.tokenSecret)
			if err != nil {
				t.Error(err)
			}
			if actualId != tc.userId {
				t.Fatalf("expected:%v VS actual:%v", tc.userId, actualId)
			}

			actualId, err = ValidateJWT("wrong token", tc.tokenSecret)
			if err == nil { // == is deliberate
				t.Error(fmt.Errorf("wrong token should fail"))
			}

			actualId, err = ValidateJWT(tokenString, "wrong secret")
			if err == nil { // == is deliberate
				t.Error(fmt.Errorf("wrong secret should fail"))
			}

			time.Sleep(tc.expiresIn + time.Second)
			actualId, err = ValidateJWT(tokenString, tc.tokenSecret)
			if err == nil { // == is deliberate
				t.Error(fmt.Errorf("too long should have expired"))
			}
		})
	}
}

// from boot.dev solution files
func TestValidateJWT(t *testing.T) {
	userID := uuid.New()
	validToken, _ := MakeJWT(userID, "secret", time.Hour)

	tests := []struct {
		name        string
		tokenString string
		tokenSecret string
		wantUserID  uuid.UUID
		wantErr     bool
	}{
		{
			name:        "Valid token",
			tokenString: validToken,
			tokenSecret: "secret",
			wantUserID:  userID,
			wantErr:     false,
		},
		{
			name:        "Invalid token",
			tokenString: "invalid.token.string",
			tokenSecret: "secret",
			wantUserID:  uuid.Nil,
			wantErr:     true,
		},
		{
			name:        "Wrong secret",
			tokenString: validToken,
			tokenSecret: "wrong_secret",
			wantUserID:  uuid.Nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUserID, err := ValidateJWT(tt.tokenString, tt.tokenSecret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJWT() error = %v, wantErr %v, gotUserId = %v",
					err,
					tt.wantErr,
					gotUserID,
				)
				return
			}
			if gotUserID != tt.wantUserID {
				t.Errorf("ValidateJWT() gotUserID = %v, want %v", gotUserID, tt.wantUserID)
			}
		})
	}
}
