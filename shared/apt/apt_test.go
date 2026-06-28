package apt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"urapt/shared/gpg"
)

func TestGenerateIndices(t *testing.T) {
	key, err := gpg.GenerateKey("urapt-test <test.example.com>", 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	suite := &Suite{
		Origin:        "urapt myrepo",
		Label:         "urapt myrepo",
		Suite:         "stable",
		Description:   "my repo",
		Components:    []string{"main", "contrib"},
		Architectures: []string{"amd64", "arm64"},
		Packages: []PackageRow{
			{
				Component: "main", Name: "foo", Version: "1.0", Architecture: "amd64",
				PoolPath: "pool/main/f/foo/foo_1.0_amd64.deb", Size: 1234,
				MD5sum: "aa", SHA1: "bb", SHA256: "cc", DescriptionMD5: "dd",
				RawControl: "Package: foo\nVersion: 1.0\nArchitecture: amd64\nDescription: short\n",
			},
			{
				Component: "main", Name: "bar", Version: "2.0", Architecture: "all",
				PoolPath: "pool/main/b/bar/bar_2.0_all.deb", Size: 5678,
				MD5sum: "ee", SHA1: "ff", SHA256: "11", DescriptionMD5: "22",
				RawControl: "Package: bar\nVersion: 2.0\nArchitecture: all\nDescription: bar short\n",
			},
		},
	}

	idx, err := Generate(suite, key)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// amd64 index should contain both foo (amd64) and bar (all).
	pkg := idx.Packages["main/binary-amd64/Packages"]
	if pkg == nil {
		t.Fatal("missing amd64 Packages")
	}
	if !bytes.Contains(pkg, []byte("Package: foo")) || !bytes.Contains(pkg, []byte("Package: bar")) {
		t.Fatalf("amd64 index missing entries:\n%s", pkg)
	}
	// bar should appear in arm64 too (arch=all).
	arm := idx.Packages["main/binary-arm64/Packages"]
	if !bytes.Contains(arm, []byte("Package: bar")) {
		t.Fatalf("arm64 index missing all-arch bar:\n%s", arm)
	}
	if bytes.Contains(arm, []byte("Package: foo")) {
		t.Fatalf("arm64 index should not contain amd64 foo:\n%s", arm)
	}
	// contrib indices should be empty bodies but present.
	if idx.Packages["contrib/binary-amd64/Packages"] == nil {
		t.Fatal("missing contrib amd64 index")
	}

	// Verify file fields appended.
	if !bytes.Contains(pkg, []byte("Filename: pool/main/f/foo/foo_1.0_amd64.deb")) {
		t.Fatalf("Packages missing Filename:\n%s", pkg)
	}
	if !bytes.Contains(pkg, []byte("SHA256: cc")) {
		t.Fatalf("Packages missing SHA256:\n%s", pkg)
	}
	if !bytes.Contains(pkg, []byte("Description-md5: dd")) {
		t.Fatalf("Packages missing Description-md5:\n%s", pkg)
	}

	// Release file should list components, architectures, and checksums.
	rel := idx.Release
	if !bytes.Contains(rel, []byte("Suite: stable")) {
		t.Fatalf("Release missing Suite:\n%s", rel)
	}
	if !bytes.Contains(rel, []byte("Components: contrib main")) {
		t.Fatalf("Release missing Components:\n%s", rel)
	}
	if !bytes.Contains(rel, []byte("Architectures: amd64 arm64")) {
		t.Fatalf("Release missing Architectures:\n%s", rel)
	}
	if !bytes.Contains(rel, []byte("SHA256:")) {
		t.Fatalf("Release missing SHA256 block:\n%s", rel)
	}
	if !bytes.Contains(rel, []byte("main/binary-amd64/Packages")) {
		t.Fatalf("Release missing index path:\n%s", rel)
	}

	// InRelease must be a clearsigned block.
	if !bytes.Contains(idx.InRelease, []byte("BEGIN PGP SIGNED MESSAGE")) {
		t.Fatalf("InRelease not clearsigned:\n%s", idx.InRelease)
	}
	// Release.gpg must be an armored detached signature.
	if !bytes.Contains(idx.ReleaseGpg, []byte("BEGIN PGP SIGNATURE")) {
		t.Fatalf("Release.gpg not armored sig:\n%s", idx.ReleaseGpg)
	}

	// Verify the clearsign and detached signatures with the public key.
	pub, err := key.ArmoredPublic()
	if err != nil {
		t.Fatalf("ArmoredPublic: %v", err)
	}
	if _, err := gpg.VerifyClearSign(pub, idx.InRelease); err != nil {
		t.Fatalf("verify InRelease: %v", err)
	}
	if err := gpg.VerifyDetached(pub, idx.Release, idx.ReleaseGpg); err != nil {
		t.Fatalf("verify Release.gpg: %v", err)
	}

	// Checksum correctness: the Release SHA256 entry for the Packages file must
	// match the bytes we generated.
	want := sha256hex(pkg)
	if !bytes.Contains(rel, []byte(" "+want)) {
		t.Fatalf("Release missing correct Packages sha256 %s:\n%s", want, rel)
	}
	_ = strings.Repeat
}

func sha256hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
