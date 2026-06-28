// Package gpg provides server-managed OpenPGP signing: key generation,
// armored export/import, clearsigning (for InRelease), and detached signing
// (for Release.gpg). It uses the pure-Go ProtonMail/go-crypto library so the
// server has no runtime dependency on the gpg binary.
package gpg

import (
	"bytes"
	"crypto"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// Key wraps an OpenPGP entity together with metadata urapt uses.
type Key struct {
	Entity      *openpgp.Entity
	Fingerprint string
	UserID      string
}

// GenerateKey creates a new RSA signing key with the given user-id (in the form
// "Name <email>" or a plain name) and key size in bits.
func GenerateKey(userID string, bits int) (*Key, error) {
	if bits <= 0 {
		bits = 4096
	}
	name, email := splitUserID(userID)
	cfg := &packet.Config{
		RSABits:     bits,
		DefaultHash: crypto.SHA256,
		V6Keys:      false,
	}
	entity, err := openpgp.NewEntity(name, "", email, cfg)
	if err != nil {
		return nil, fmt.Errorf("new entity: %w", err)
	}
	return &Key{
		Entity:      entity,
		Fingerprint: fmt.Sprintf("%X", entity.PrimaryKey.Fingerprint),
		UserID:      userID,
	}, nil
}

// ParseArmoredPrivate decodes an ASCII-armored private key produced by
// ArmoredPrivate.
func ParseArmoredPrivate(armored string) (*Key, error) {
	block, err := armor.Decode(strings.NewReader(armored))
	if err != nil {
		return nil, fmt.Errorf("decode armor: %w", err)
	}
	if block.Type != "PGP PRIVATE KEY BLOCK" {
		return nil, fmt.Errorf("unexpected armor type %q", block.Type)
	}
	entity, err := openpgp.ReadEntity(packet.NewReader(block.Body))
	if err != nil {
		return nil, fmt.Errorf("read entity: %w", err)
	}
	uid := ""
	if id := entity.PrimaryIdentity(); id != nil {
		uid = id.Name
	}
	return &Key{
		Entity:      entity,
		Fingerprint: fmt.Sprintf("%X", entity.PrimaryKey.Fingerprint),
		UserID:      uid,
	}, nil
}

// ArmoredPublic returns the ASCII-armored public key.
func (k *Key) ArmoredPublic() (string, error) {
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, "PGP PUBLIC KEY BLOCK", nil)
	if err != nil {
		return "", fmt.Errorf("armor encode: %w", err)
	}
	if err := k.Entity.Serialize(w); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("serialize public: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close armor: %w", err)
	}
	return buf.String(), nil
}

// ArmoredPrivate returns the ASCII-armored private key (unencrypted).
func (k *Key) ArmoredPrivate() (string, error) {
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, "PGP PRIVATE KEY BLOCK", nil)
	if err != nil {
		return "", fmt.Errorf("armor encode: %w", err)
	}
	if err := k.Entity.SerializePrivate(w, nil); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("serialize private: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close armor: %w", err)
	}
	return buf.String(), nil
}

// ClearSign produces a clearsigned message (used for the InRelease file).
func (k *Key) ClearSign(data []byte) ([]byte, error) {
	sk, ok := k.Entity.SigningKey(time.Now())
	if !ok {
		return nil, fmt.Errorf("no signing key available")
	}
	var out bytes.Buffer
	cfg := &packet.Config{DefaultHash: crypto.SHA256}
	plaintext, err := clearsign.Encode(&out, sk.PrivateKey, cfg)
	if err != nil {
		return nil, fmt.Errorf("clearsign encode: %w", err)
	}
	if _, err := plaintext.Write(data); err != nil {
		_ = plaintext.Close()
		return nil, fmt.Errorf("write clearsign: %w", err)
	}
	if err := plaintext.Close(); err != nil {
		return nil, fmt.Errorf("close clearsign: %w", err)
	}
	return out.Bytes(), nil
}

// DetachedSign produces an ASCII-armored detached signature of data (used for
// Release.gpg).
func (k *Key) DetachedSign(data []byte) ([]byte, error) {
	var out bytes.Buffer
	cfg := &packet.Config{DefaultHash: crypto.SHA256}
	if err := openpgp.ArmoredDetachSign(&out, k.Entity, bytes.NewReader(data), cfg); err != nil {
		return nil, fmt.Errorf("detach sign: %w", err)
	}
	return out.Bytes(), nil
}

// VerifyClearSign is a test helper that verifies a clearsigned block and
// returns the plaintext.
func VerifyClearSign(armoredPublic string, clearsigned []byte) (plaintext []byte, err error) {
	key, err := ParseArmoredPublic(armoredPublic)
	if err != nil {
		return nil, err
	}
	block, rest := clearsign.Decode(clearsigned)
	if block == nil {
		return nil, fmt.Errorf("no clearsign block (rest=%d bytes)", len(rest))
	}
	keyring := openpgp.EntityList{key.Entity}
	if _, err := block.VerifySignature(keyring, nil); err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	return block.Bytes, nil
}

// VerifyDetached verifies an armored detached signature of data using the
// given armored public key. Test helper.
func VerifyDetached(armoredPublic string, data, armoredSig []byte) error {
	key, err := ParseArmoredPublic(armoredPublic)
	if err != nil {
		return err
	}
	keyring := openpgp.EntityList{key.Entity}
	if _, err := openpgp.CheckArmoredDetachedSignature(keyring, bytes.NewReader(data), bytes.NewReader(armoredSig), nil); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	return nil
}

// ParseArmoredPublic decodes an ASCII-armored public key.
func ParseArmoredPublic(armored string) (*Key, error) {
	block, err := armor.Decode(strings.NewReader(armored))
	if err != nil {
		return nil, fmt.Errorf("decode armor: %w", err)
	}
	if block.Type != "PGP PUBLIC KEY BLOCK" {
		return nil, fmt.Errorf("unexpected armor type %q", block.Type)
	}
	entity, err := openpgp.ReadEntity(packet.NewReader(block.Body))
	if err != nil {
		return nil, fmt.Errorf("read entity: %w", err)
	}
	uid := ""
	if id := entity.PrimaryIdentity(); id != nil {
		uid = id.Name
	}
	return &Key{
		Entity:      entity,
		Fingerprint: fmt.Sprintf("%X", entity.PrimaryKey.Fingerprint),
		UserID:      uid,
	}, nil
}

// splitUserID parses a user-id of the form "Name <email>" into name and email.
// If no email brackets are present, the whole string is treated as the name.
func splitUserID(userID string) (name, email string) {
	userID = strings.TrimSpace(userID)
	i := strings.LastIndexByte(userID, '<')
	j := strings.LastIndexByte(userID, '>')
	if i >= 0 && j > i {
		name = strings.TrimSpace(userID[:i])
		email = strings.TrimSpace(userID[i+1 : j])
		return name, email
	}
	return userID, ""
}

// ensure io is referenced (used implicitly by armor/clearsign APIs).
var _ = io.EOF
