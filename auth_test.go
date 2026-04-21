package main

import (
	"testing"
	"time"
)

func TestAuthDataIsExpired(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"zero value is not expired", time.Time{}, false},
		{"future is not expired", time.Now().Add(time.Hour), false},
		{"past is expired", time.Now().Add(-time.Hour), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &AuthData{ExpiresAt: tc.at}
			if got := a.isExpired(); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
