package apiclient_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
)

func newHTTPServer(h http.Handler) *httptest.Server {
	return httptest.NewServer(h)
}

// buildDeb constructs a minimal valid .deb with the given package/version/arch.
func buildDeb(name, version, arch string) []byte {
	control := "Package: " + name + "\nVersion: " + version + "\nArchitecture: " + arch + "\nMaintainer: t <t@e>\nDescription: short\n extended\n"
	var ctrlBuf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&ctrlBuf, 9)
	tw := tar.NewWriter(gz)
	writeTw(tw, "control", control)
	tw.Close()
	gz.Close()

	var dataBuf bytes.Buffer
	gz2, _ := gzip.NewWriterLevel(&dataBuf, 9)
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
	copy(header[48:], []byte(padNum(len(data), 10)))
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
