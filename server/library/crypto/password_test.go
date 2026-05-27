package crypto

import "testing"

func TestHashPasswordVerifiesOriginalPasswordOnly(t *testing.T) {
	hash, err := HashPassword("papermind123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "papermind123" || hash == "" {
		t.Fatalf("hash should not expose raw password: %q", hash)
	}
	if !VerifyPassword(hash, "papermind123") {
		t.Fatalf("VerifyPassword() rejected original password")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatalf("VerifyPassword() accepted wrong password")
	}
}

func TestVerifyPasswordRejectsPlaintextAndMalformedHashes(t *testing.T) {
	for _, hash := range []string{"papermind123", "", "pbkdf2_sha256$bad$salt$key"} {
		if VerifyPassword(hash, "papermind123") {
			t.Fatalf("VerifyPassword(%q) = true, want false", hash)
		}
	}
}
