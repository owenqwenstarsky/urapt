package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeTestDeb builds a minimal valid .deb at path with the given control
// stanza text.
func makeTestDeb(t *testing.T, path, controlText string) {
	t.Helper()
	controlTar := buildControlTar(t, controlText)
	dataTar := buildDataTar(t)
	deb := buildAr(t, controlTar, dataTar)
	if err := os.WriteFile(path, deb, 0o644); err != nil {
		t.Fatalf("write deb: %v", err)
	}
}

func buildControlTar(t *testing.T, controlText string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	addFile(t, tw, "control", controlText)
	if err := tw.Close(); err != nil {
		t.Fatalf("close control tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close control gz: %v", err)
	}
	return buf.Bytes()
}

func buildDataTar(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	addFile(t, tw, "usr/share/doc/foo/README", "readme\n")
	if err := tw.Close(); err != nil {
		t.Fatalf("close data tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close data gz: %v", err)
	}
	return buf.Bytes()
}

func addFile(t *testing.T, tw *tar.Writer, name, body string) {
	t.Helper()
	if err := tw.WriteHeader(&tar.Header{
		Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("write tar header %s: %v", name, err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatalf("write tar body %s: %v", name, err)
	}
}

func buildAr(t *testing.T, controlTar, dataTar []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("!<arch>\n")
	writeArMember(&buf, "debian-binary", []byte("2.0\n"))
	writeArMember(&buf, "control.tar.gz", controlTar)
	writeArMember(&buf, "data.tar.gz", dataTar)
	return buf.Bytes()
}

func writeArMember(buf *bytes.Buffer, name string, data []byte) {
	header := make([]byte, 60)
	for i := range header {
		header[i] = ' '
	}
	copy(header[0:], name+"/")
	copy(header[48:], []byte(padLeft(len(data), 10)))
	header[58] = '`'
	header[59] = '\n'
	buf.Write(header)
	buf.Write(data)
	if len(data)%2 == 1 {
		buf.WriteByte('\n')
	}
}

func padLeft(n, width int) string {
	s := []byte(strings.Repeat(" ", width))
	v := []byte(itoa(n))
	copy(s[len(s)-len(v):], v)
	return string(s)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestInspect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo_1.0_amd64.deb")
	controlText := `Package: foo
Version: 1.0
Architecture: amd64
Maintainer: Test <test@example.com>
Installed-Size: 42
Depends: libc6 (>= 2.31), bash | dash
Section: utils
Priority: optional
Homepage: https://example.com
Description: short summary
 extended description line one
 .
 second paragraph.
`
	makeTestDeb(t, path, controlText)

	d, err := Inspect(path)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if d.Control.Get("Package") != "foo" {
		t.Fatalf("Package = %q", d.Control.Get("Package"))
	}
	if d.Control.Get("Version") != "1.0" {
		t.Fatalf("Version = %q", d.Control.Get("Version"))
	}
	if d.Control.Get("Architecture") != "amd64" {
		t.Fatalf("Architecture = %q", d.Control.Get("Architecture"))
	}
	if d.Control.Get("Depends") != "libc6 (>= 2.31), bash | dash" {
		t.Fatalf("Depends = %q", d.Control.Get("Depends"))
	}
	if d.Size <= 0 {
		t.Fatalf("Size = %d", d.Size)
	}
	if d.MD5sum == "" || d.SHA1 == "" || d.SHA256 == "" {
		t.Fatalf("hashes empty: md5=%s sha1=%s sha256=%s", d.MD5sum, d.SHA1, d.SHA256)
	}
	if d.Control.ShortDescription() != "short summary" {
		t.Fatalf("short = %q", d.Control.ShortDescription())
	}
	long := d.Control.LongDescription()
	if !strings.Contains(long, "extended description line one") || !strings.Contains(long, "second paragraph.") {
		t.Fatalf("long = %q", long)
	}
	if d.Control.DescriptionMD5() == "" {
		t.Fatal("empty description md5")
	}
}

func TestParseControlContinuation(t *testing.T) {
	c, err := ParseControl(strings.NewReader("Package: bar\nVersion: 1\nDescription: short\n long\n .\n more\n"))
	if err != nil {
		t.Fatalf("ParseControl: %v", err)
	}
	if c.Get("Description") != "short\nlong\n\nmore" {
		t.Fatalf("Description = %q", c.Get("Description"))
	}
}
