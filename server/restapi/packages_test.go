package restapi

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// buildDebBytes constructs a minimal valid .deb with the given control text.
func buildDebBytes(t *testing.T, controlText string) []byte {
	t.Helper()
	var ctrlBuf bytes.Buffer
	gz := gzip.NewWriter(&ctrlBuf)
	tw := tar.NewWriter(gz)
	writeTarFile(t, tw, "control", controlText)
	tw.Close()
	gz.Close()

	var dataBuf bytes.Buffer
	gz2 := gzip.NewWriter(&dataBuf)
	tw2 := tar.NewWriter(gz2)
	writeTarFile(t, tw2, "usr/share/foo", "x")
	tw2.Close()
	gz2.Close()

	var out bytes.Buffer
	out.WriteString("!<arch>\n")
	writeArMemberBytes(&out, "debian-binary", []byte("2.0\n"))
	writeArMemberBytes(&out, "control.tar.gz", ctrlBuf.Bytes())
	writeArMemberBytes(&out, "data.tar.gz", dataBuf.Bytes())
	return out.Bytes()
}

func writeTarFile(t *testing.T, tw *tar.Writer, name, body string) {
	t.Helper()
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatalf("tar write: %v", err)
	}
}

func writeArMemberBytes(buf *bytes.Buffer, name string, data []byte) {
	header := make([]byte, 60)
	for i := range header {
		header[i] = ' '
	}
	copy(header[0:], name+"/")
	ds := []byte(padLeftInt(len(data), 10))
	copy(header[48:], ds)
	header[58] = '`'
	header[59] = '\n'
	buf.Write(header)
	buf.Write(data)
	if len(data)%2 == 1 {
		buf.WriteByte('\n')
	}
}

func padLeftInt(n, width int) string {
	s := make([]byte, width)
	for i := range s {
		s[i] = ' '
	}
	digits := []byte(itoaInt(n))
	copy(s[len(s)-len(digits):], digits)
	return string(s)
}

func itoaInt(n int) string {
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

// uploadPackage POSTs a multipart push.
func (h *harness) uploadPackage(token, repo, dist, component string, deb []byte) (int, []byte) {
	h.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("component", component)
	fw, _ := mw.CreateFormFile("file", "foo_1.0_amd64.deb")
	fw.Write(deb)
	mw.Close()

	req, _ := http.NewRequest("POST", h.srv.URL+"/api/v1/repositories/"+repo+"/distributions/"+dist+"/packages", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func TestPushPullDeletePackage(t *testing.T) {
	h := newHarness(t)
	_, body := h.do("POST", "/api/v1/auth/register", "", map[string]string{"username": "owner", "password": "supersecret"})
	var owner struct {
		Token string `json:"token"`
	}
	json.Unmarshal(body, &owner)

	// Set packages dir so blobs land in the temp dir.
	_ = h.store // packages dir is taken from config (temp dir in harness).

	h.do("POST", "/api/v1/repositories", owner.Token, map[string]any{"name": "pkgrepo", "visibility": "public"})
	h.do("POST", "/api/v1/repositories/pkgrepo/distributions", owner.Token, map[string]string{"name": "stable"})
	h.do("POST", "/api/v1/repositories/pkgrepo/distributions/stable/components", owner.Token, map[string]string{"name": "main"})
	h.do("POST", "/api/v1/repositories/pkgrepo/distributions/stable/architectures", owner.Token, map[string]string{"name": "amd64"})

	debBytes := buildDebBytes(t, "Package: foo\nVersion: 1.0\nArchitecture: amd64\nMaintainer: Test <t@e.com>\nDescription: short\n extended\n")

	code, body := h.uploadPackage(owner.Token, "pkgrepo", "stable", "main", debBytes)
	if code != 201 {
		t.Fatalf("push status %d body %s", code, body)
	}
	var pkg struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Pool string `json:"pool_path"`
		SHA  string `json:"sha256"`
	}
	json.Unmarshal(body, &pkg)
	if pkg.Name != "foo" || pkg.SHA == "" {
		t.Fatalf("unexpected pkg: %+v", pkg)
	}

	// duplicate push should 409
	code, _ = h.uploadPackage(owner.Token, "pkgrepo", "stable", "main", debBytes)
	if code != 409 {
		t.Fatalf("duplicate push should be 409, got %d", code)
	}

	// list
	code, body = h.do("GET", "/api/v1/repositories/pkgrepo/distributions/stable/packages", owner.Token, nil)
	if code != 200 {
		t.Fatalf("list packages status %d", code)
	}

	// get
	code, _ = h.do("GET", "/api/v1/repositories/pkgrepo/packages/"+pkg.ID, owner.Token, nil)
	if code != 200 {
		t.Fatalf("get package status %d", code)
	}

	// download file
	code, body = h.do("GET", "/api/v1/repositories/pkgrepo/packages/"+pkg.ID+"/file", owner.Token, nil)
	if code != 200 {
		t.Fatalf("get file status %d", code)
	}
	if !bytes.Equal(body, debBytes) {
		t.Fatalf("downloaded file does not match uploaded (%d vs %d bytes)", len(body), len(debBytes))
	}

	// verify blob file exists on disk in the temp packages dir
	blobPath := filepath.Join(h.pkgDir, pkg.SHA+".deb")
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("blob file missing on disk: %v", err)
	}

	// delete
	code, _ = h.do("DELETE", "/api/v1/repositories/pkgrepo/packages/"+pkg.ID, owner.Token, nil)
	if code != 204 {
		t.Fatalf("delete package status %d", code)
	}
	// get now 404
	code, _ = h.do("GET", "/api/v1/repositories/pkgrepo/packages/"+pkg.ID, owner.Token, nil)
	if code != 404 {
		t.Fatalf("deleted package should 404, got %d", code)
	}
	// blob file removed after refcount hits 0
	if _, err := os.Stat(blobPath); !os.IsNotExist(err) {
		t.Fatalf("blob file should be removed after delete, err=%v", err)
	}
}
