package aptrepo_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func jsonMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func readAll(r io.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}

func mkdirAll(dir string) error { return os.MkdirAll(dir, 0o755) }

func str(body []byte, key string) string {
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// buildDeb constructs a minimal valid .deb with the given package/version/arch.
func buildDeb(name, version, arch string) []byte {
	control := "Package: " + name + "\nVersion: " + version + "\nArchitecture: " + arch + "\nMaintainer: t <t@e>\nDescription: short\n extended\n"
	var ctrlBuf bytes.Buffer
	gz := gzip.NewWriter(&ctrlBuf)
	tw := tar.NewWriter(gz)
	writeTw(tw, "control", control)
	tw.Close()
	gz.Close()

	var dataBuf bytes.Buffer
	gz2 := gzip.NewWriter(&dataBuf)
	tw2 := tar.NewWriter(gz2)
	writeTw(tw2, "usr/share/"+name, "x")
	tw2.Close()
	gz2.Close()

	var out bytes.Buffer
	out.WriteString("!<arch>\n")
	writeAr(&out, "debian-binary", []byte("2.0\n"))
	writeAr(&out, "control.tar.gz", ctrlBuf.Bytes())
	writeAr(&out, "data.tar.gz", dataBuf.Bytes())
	return out.Bytes()
}

func writeTw(tw *tar.Writer, name, body string) {
	_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte(body))
}

func writeAr(buf *bytes.Buffer, name string, data []byte) {
	header := make([]byte, 60)
	for i := range header {
		header[i] = ' '
	}
	copy(header[0:], name+"/")
	ds := []byte(padNum(len(data), 10))
	copy(header[48:], ds)
	header[58] = '`'
	header[59] = '\n'
	buf.Write(header)
	buf.Write(data)
	if len(data)%2 == 1 {
		buf.WriteByte('\n')
	}
}

func padNum(n, width int) string {
	s := make([]byte, width)
	for i := range s {
		s[i] = ' '
	}
	digits := []byte(itoa(n))
	copy(s[len(s)-len(digits):], digits)
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

func upload(t *testing.T, srv *httptest.Server, token, repo, dist, component, filename string, deb []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("component", component)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(deb)
	_ = mw.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/repositories/"+repo+"/distributions/"+dist+"/packages", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("upload status %d", resp.StatusCode)
	}
}
