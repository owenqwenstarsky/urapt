package crypto

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	h, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(h, "hunter2") {
		t.Fatal("expected verify to pass for correct password")
	}
	if VerifyPassword(h, "wrong") {
		t.Fatal("expected verify to fail for wrong password")
	}
}

func TestGenerateToken(t *testing.T) {
	tok, hash, prefix, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if tok == "" || hash == "" || prefix == "" {
		t.Fatal("empty token fields")
	}
	if len(tok) <= len(TokenPrefix) {
		t.Fatal("token too short")
	}
	if tok[:len(TokenPrefix)] != TokenPrefix {
		t.Fatalf("token missing prefix: %q", tok)
	}
	if HashToken(tok) != hash {
		t.Fatal("HashToken does not match returned hash")
	}
	if prefix != tok[:len(TokenPrefix)+8] {
		t.Fatalf("prefix %q != expected", prefix)
	}

	tok2, _, _, _ := GenerateToken()
	if tok == tok2 {
		t.Fatal("expected distinct tokens")
	}
}
