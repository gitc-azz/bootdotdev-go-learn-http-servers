package auth

import (
	"testing"
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
