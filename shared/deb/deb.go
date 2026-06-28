// Package deb parses Debian .deb archives: it reads the ar container, locates
// and decompresses the control.tar.* member, parses the control stanza, and
// computes whole-file hashes/sizes. It performs no execution of package
// contents.
package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// Deb holds the parsed metadata of a .deb file.
type Deb struct {
	Control Control
	Size    int64
	MD5sum  string
	SHA1    string
	SHA256  string
}

// Field is a single control header field, preserving original key casing.
type Field struct {
	Key   string
	Value string
}

// Control is a parsed control stanza. Fields preserves insertion order;
// Lookup gives case-insensitive access by key. Raw is the original text.
type Control struct {
	Fields []Field
	Lookup map[string]string
	Raw    string
}

// Get returns the value of a field (case-insensitive), or "" if absent.
func (c Control) Get(key string) string {
	if c.Lookup == nil {
		return ""
	}
	return c.Lookup[strings.ToLower(key)]
}

// Inspect opens the .deb at path, computes whole-file hashes/size, and parses
// its control stanza.
func Inspect(path string) (*Deb, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open deb: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat deb: %w", err)
	}

	hMD5 := md5.New()
	hSHA1 := sha1.New()
	hSHA256 := sha256.New()
	size := stat.Size()

	if _, err := io.Copy(io.MultiWriter(hMD5, hSHA1, hSHA256), f); err != nil {
		return nil, fmt.Errorf("hash deb: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek deb: %w", err)
	}

	control, err := readControlFromAR(f)
	if err != nil {
		return nil, err
	}

	return &Deb{
		Control: control,
		Size:    size,
		MD5sum:  hex.EncodeToString(hMD5.Sum(nil)),
		SHA1:    hex.EncodeToString(hSHA1.Sum(nil)),
		SHA256:  hex.EncodeToString(hSHA256.Sum(nil)),
	}, nil
}

// readControlFromAR reads an ar stream and returns the parsed control stanza.
func readControlFromAR(r io.Reader) (Control, error) {
	members, err := readAR(r)
	if err != nil {
		return Control{}, err
	}
	var debianBinary []byte
	var controlTar []byte
	var controlTarName string
	for _, m := range members {
		switch {
		case m.Name == "debian-binary":
			debianBinary = m.Data
		case strings.HasPrefix(m.Name, "control.tar"):
			controlTar = m.Data
			controlTarName = m.Name
		}
	}
	if debianBinary == nil {
		return Control{}, fmt.Errorf("missing debian-binary member")
	}
	if !bytes.HasPrefix(bytes.TrimSpace(debianBinary), []byte("2.0")) {
		return Control{}, fmt.Errorf("unsupported deb format (expected 2.0)")
	}
	if controlTar == nil {
		return Control{}, fmt.Errorf("missing control.tar member")
	}

	decompressed, err := decompressMember(controlTarName, controlTar)
	if err != nil {
		return Control{}, err
	}
	return parseControlTar(decompressed)
}

// arMember is a single file within an ar archive.
type arMember struct {
	Name string
	Data []byte
}

// readAR parses a Unix ar archive (the variant used by .deb: GNU/SysV style,
// with member names terminated by '/' and no long-name index needed for the
// short standard member names).
func readAR(r io.Reader) ([]arMember, error) {
	br := newBlockReader(r)
	magic := make([]byte, 8)
	if _, err := io.ReadFull(br, magic); err != nil {
		return nil, fmt.Errorf("read ar magic: %w", err)
	}
	if string(magic) != "!<arch>\n" {
		return nil, fmt.Errorf("not an ar archive (bad magic)")
	}

	var members []arMember
	for {
		header := make([]byte, 60)
		n, err := io.ReadFull(br, header)
		if err == io.EOF || (err == io.ErrUnexpectedEOF && n == 0) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read ar header: %w", err)
		}
		if string(header[58:60]) != "`\n" {
			return nil, fmt.Errorf("bad ar header terminator")
		}
		name := strings.TrimSpace(strings.TrimRight(string(header[0:16]), " "))
		name = strings.TrimSuffix(name, "/")
		sizeStr := strings.TrimSpace(string(header[48:58]))
		var size int64
		if _, err := fmt.Sscanf(sizeStr, "%d", &size); err != nil {
			return nil, fmt.Errorf("parse ar member size %q: %w", sizeStr, err)
		}
		if size < 0 {
			return nil, fmt.Errorf("negative ar member size")
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(br, data); err != nil {
			return nil, fmt.Errorf("read ar member %q: %w", name, err)
		}
		members = append(members, arMember{Name: name, Data: data})
		if size%2 == 1 {
			pad := make([]byte, 1)
			if _, err := io.ReadFull(br, pad); err != nil {
				return nil, fmt.Errorf("read ar padding: %w", err)
			}
		}
	}
	return members, nil
}

// blockReader is a thin wrapper that ensures io.ReadFull semantics work on any
// io.Reader (it just forwards reads).
type blockReader struct {
	r io.Reader
}

func newBlockReader(r io.Reader) *blockReader     { return &blockReader{r: r} }
func (b *blockReader) Read(p []byte) (int, error) { return b.r.Read(p) }

// decompressMember decompresses a control.tar.* member based on its name
// extension.
func decompressMember(name string, data []byte) ([]byte, error) {
	switch {
	case strings.HasSuffix(name, ".gz"):
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close()
		return io.ReadAll(gz)
	case strings.HasSuffix(name, ".xz"):
		xr, err := xz.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("xz: %w", err)
		}
		return io.ReadAll(xr)
	case strings.HasSuffix(name, ".zst"):
		zr, err := zstd.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("zstd: %w", err)
		}
		defer zr.Close()
		return io.ReadAll(zr)
	case strings.HasSuffix(name, ".tar"):
		return data, nil
	default:
		return nil, fmt.Errorf("unknown control.tar compression: %s", name)
	}
}

// parseControlTar reads a tar stream and returns the first control stanza
// found in a file named "control" or "./control".
func parseControlTar(tarData []byte) (Control, error) {
	tr := tar.NewReader(bytes.NewReader(tarData))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Control{}, fmt.Errorf("read tar: %w", err)
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if name == "control" {
			body, err := io.ReadAll(tr)
			if err != nil {
				return Control{}, fmt.Errorf("read control: %w", err)
			}
			return ParseControl(bytes.NewReader(body))
		}
	}
	return Control{}, fmt.Errorf("control file not found in control.tar")
}

// ParseControl parses a single RFC822-style control stanza. Continuation
// lines (starting with a space or tab) are appended to the previous field's
// value with the leading whitespace preserved as a single space.
func ParseControl(r io.Reader) (Control, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Control{}, fmt.Errorf("read control: %w", err)
	}
	raw := strings.TrimRight(string(data), "\n")
	c := Control{Lookup: map[string]string{}, Raw: raw}

	var curKey, curVal string
	flush := func() {
		if curKey == "" {
			return
		}
		c.Fields = append(c.Fields, Field{Key: curKey, Value: curVal})
		c.Lookup[strings.ToLower(curKey)] = curVal
		curKey, curVal = "", ""
	}

	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			flush()
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			if curKey == "" {
				return Control{}, fmt.Errorf("continuation line with no field")
			}
			// Debian control continuation: strip exactly one leading space,
			// and a line that is just "." represents a blank line.
			body := line
			if body[0] == ' ' {
				body = body[1:]
			} else {
				body = strings.TrimLeft(body, " \t")
			}
			if body == "." {
				curVal += "\n"
			} else {
				curVal += "\n" + strings.TrimRight(body, " \t\r")
			}
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			return Control{}, fmt.Errorf("malformed control line: %q", line)
		}
		flush()
		curKey = strings.TrimSpace(line[:colon])
		curVal = strings.TrimSpace(line[colon+1:])
	}
	flush()

	if len(c.Fields) == 0 {
		return Control{}, fmt.Errorf("empty control stanza")
	}
	return c, nil
}

// ShortDescription returns the short summary (the first line of Description).
func (c Control) ShortDescription() string {
	desc := c.Get("Description")
	if desc == "" {
		return ""
	}
	if i := strings.IndexByte(desc, '\n'); i >= 0 {
		return desc[:i]
	}
	return desc
}

// LongDescription returns the extended description (everything after the short
// summary line).
func (c Control) LongDescription() string {
	desc := c.Get("Description")
	if desc == "" {
		return ""
	}
	if i := strings.IndexByte(desc, '\n'); i >= 0 {
		return strings.TrimLeft(desc[i+1:], "\n")
	}
	return ""
}

// DescriptionMD5 returns the MD5 hex digest of the long description, matching
// Debian's Description-md5 Packages field.
func (c Control) DescriptionMD5() string {
	sum := md5.Sum([]byte(c.LongDescription()))
	return hex.EncodeToString(sum[:])
}

// IsControlFieldName reports whether s looks like a valid control field name
// (non-empty, colon-free, starts with a non-space, ASCII letters/digits/-).
func IsControlFieldName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' {
			continue
		}
		return false
	}
	return true
}

// ensure unicode is referenced (reserved for stricter validation later).
var _ = unicode.IsLetter
