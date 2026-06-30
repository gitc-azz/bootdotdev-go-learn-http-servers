package main

import (
	"reflect"
	"testing"
)

func TestCensorship(t *testing.T) {
	tcs := map[string]struct {
		input    chirp
		expected chirp
	}{
		"nothing_to_change": {
			input:    chirp{Body: "I a positive comment on the world"},
			expected: chirp{Body: "I a positive comment on the world"},
		},
		"one_to_change_01": {
			input:    chirp{Body: "kerfuffle I a positive comment on the world"},
			expected: chirp{Body: "**** I a positive comment on the world"},
		},
		"two_to_change_01": {
			input:    chirp{Body: "kerfuffle I a positive sharbert comment on the world"},
			expected: chirp{Body: "**** I a positive **** comment on the world"},
		},
		"three_to_change_01": {
			input:    chirp{Body: "kerfuffle I a positive sharbert comment on the world fornax"},
			expected: chirp{Body: "**** I a positive **** comment on the world ****"},
		},
		"two_to_change_one_to_keep_01": {
			input:    chirp{Body: "kerfuffle! I a positive sharbert comment on the world fornax"},
			expected: chirp{Body: "kerfuffle! I a positive **** comment on the world ****"},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			actual := censorship(tc.input)
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Fatalf("actual: %v, expected: %v", actual, tc.expected)
			}
		})
	}
}
