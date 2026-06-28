package gpg

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenerateAndSign(t *testing.T) {
	k, err := GenerateKey("urapt-server <test.example.com>", 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if k.Fingerprint == "" {
		t.Fatal("empty fingerprint")
	}

	pub, err := k.ArmoredPublic()
	if err != nil {
		t.Fatalf("ArmoredPublic: %v", err)
	}
	if !bytes.Contains([]byte(pub), []byte("BEGIN PGP PUBLIC KEY BLOCK")) {
		t.Fatal("bad armored public")
	}
	priv, err := k.ArmoredPrivate()
	if err != nil {
		t.Fatalf("ArmoredPrivate: %v", err)
	}
	if !bytes.Contains([]byte(priv), []byte("BEGIN PGP PRIVATE KEY BLOCK")) {
		t.Fatal("bad armored private")
	}

	data := []byte("Origin: urapt\nSuite: stable\n\nContents here.\n")

	clear, err := k.ClearSign(data)
	if err != nil {
		t.Fatalf("ClearSign: %v", err)
	}
	if !bytes.Contains(clear, []byte("BEGIN PGP SIGNED MESSAGE")) {
		t.Fatal("bad clearsign output")
	}
	pt, err := VerifyClearSign(pub, clear)
	if err != nil {
		t.Fatalf("VerifyClearSign: %v", err)
	}
	if strings.ReplaceAll(string(pt), "\r\n", "\n") != string(data) {
		t.Fatalf("plaintext mismatch: got %q want %q", pt, data)
	}

	det, err := k.DetachedSign(data)
	if err != nil {
		t.Fatalf("DetachedSign: %v", err)
	}
	if !bytes.Contains(det, []byte("BEGIN PGP SIGNATURE")) {
		t.Fatal("bad detached output")
	}
	if err := VerifyDetached(pub, data, det); err != nil {
		t.Fatalf("VerifyDetached: %v", err)
	}

	k2, err := ParseArmoredPrivate(priv)
	if err != nil {
		t.Fatalf("ParseArmoredPrivate: %v", err)
	}
	if k2.Fingerprint != k.Fingerprint {
		t.Fatalf("fingerprint mismatch after round-trip: %s != %s", k2.Fingerprint, k.Fingerprint)
	}
}
