package auth // Change this to match your actual package name

import (
	"testing"
	"time"

	"github.com/google/uuid"
	// Ensure you have this imported if you aren't already
	// "github.com/golang-jwt/jwt/v5" 
)

func TestJWTGeneratonAndValidation(t *testing.T) {
	secret := "my-super-secret-test-key"
	userID := uuid.New()

	t.Run("Valid Token - Happy Path", func(t *testing.T) {
		token, err := MakeJWT(userID, secret, time.Hour)
		if err != nil {
			t.Fatalf("MakeJWT failed: unexpected error: %v", err)
		}
		if token == "" {
			t.Fatal("MakeJWT failed: returned empty token string")
		}

		parsedID, err := ValidateJWT(token, secret)
		if err != nil {
			t.Fatalf("ValidateJWT failed: unexpected error: %v", err)
		}
		if parsedID != userID {
			t.Errorf("ValidateJWT failed: expected userID %v, got %v", userID, parsedID)
		}
	})

	t.Run("Invalid Secret", func(t *testing.T) {
		token, err := MakeJWT(userID, secret, time.Hour)
		if err != nil {
			t.Fatalf("MakeJWT setup failed: %v", err)
		}

		_, err = ValidateJWT(token, "the-wrong-secret")
		if err == nil {
			t.Fatal("ValidateJWT failed: expected an error when using an incorrect secret, got nil")
		}
	})

	t.Run("Expired Token", func(t *testing.T) {
		// Pass a negative duration to ensure it expires instantly
		token, err := MakeJWT(userID, secret, -time.Second)
		if err != nil {
			t.Fatalf("MakeJWT setup failed: %v", err)
		}

		_, err = ValidateJWT(token, secret)
		if err == nil {
			t.Fatal("ValidateJWT failed: expected an error for an expired token, got nil")
		}
	})

	t.Run("Malformed Token String", func(t *testing.T) {
		invalidToken := "this.is.not.a.valid.jwt"
		
		_, err := ValidateJWT(invalidToken, secret)
		if err == nil {
			t.Fatal("ValidateJWT failed: expected an error for a malformed token string, got nil")
		}
	})
}
