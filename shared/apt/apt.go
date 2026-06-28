// Package apt generates APT repository indices (Packages, Release, InRelease,
// Release.gpg) entirely from in-memory package data. Indices are never written
// to disk by this package; the caller caches and serves them.
package apt

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

// Signer is implemented by anything able to clearsign and detached-sign the
// Release file (e.g. *gpg.Key).
type Signer interface {
	ClearSign(data []byte) ([]byte, error)
	DetachedSign(data []byte) ([]byte, error)
}

// PackageRow is a single package's data needed to emit its index entry.
type PackageRow struct {
	Component      string
	Name           string
	Version        string
	Architecture   string
	PoolPath       string
	Size           int64
	MD5sum         string
	SHA1           string
	SHA256         string
	DescriptionMD5 string
	RawControl     string
}

// Suite describes a (repository, distribution) for which to generate indices.
type Suite struct {
	Origin        string
	Label         string
	Suite         string
	Codename      string
	Description   string
	Components    []string
	Architectures []string
	Packages      []PackageRow
}

// Indices holds all generated index artifacts for a suite, keyed by their path
// relative to the suite directory.
type Indices struct {
	Packages   map[string][]byte
	PackagesGz map[string][]byte
	PackagesXz map[string][]byte
	Release    []byte
	InRelease  []byte
	ReleaseGpg []byte
}

// Generate builds all indices for the suite and signs the Release file using
// signer. If signer is nil, InRelease/ReleaseGpg are left empty.
func Generate(s *Suite, signer Signer) (*Indices, error) {
	components := dedupSorted(s.Components)
	arches := dedupSorted(s.Architectures)

	idx := &Indices{
		Packages:   map[string][]byte{},
		PackagesGz: map[string][]byte{},
		PackagesXz: map[string][]byte{},
	}

	// Group packages by (component, arch), including arch="all" in every arch.
	type key struct{ comp, arch string }
	groups := map[key][]PackageRow{}
	for _, p := range s.Packages {
		for _, arch := range arches {
			if p.Architecture == arch || p.Architecture == "all" {
				k := key{p.Component, arch}
				groups[k] = append(groups[k], p)
			}
		}
	}

	for _, comp := range components {
		for _, arch := range arches {
			rows := groups[key{comp, arch}]
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Name != rows[j].Name {
					return rows[i].Name < rows[j].Name
				}
				return rows[i].Version < rows[j].Version
			})
			pkgBytes := generatePackagesIndex(rows)
			relPath := fmt.Sprintf("%s/binary-%s/Packages", comp, arch)
			idx.Packages[relPath] = pkgBytes
			idx.PackagesGz[relPath+".gz"] = gzipBytes(pkgBytes)
			xzBytes, err := xzBytes(pkgBytes)
			if err != nil {
				return nil, fmt.Errorf("xz compress %s: %w", relPath, err)
			}
			idx.PackagesXz[relPath+".xz"] = xzBytes
		}
	}

	release, err := generateRelease(s, components, arches, idx)
	if err != nil {
		return nil, err
	}
	idx.Release = release

	if signer != nil {
		clear, err := signer.ClearSign(release)
		if err != nil {
			return nil, fmt.Errorf("clearsign: %w", err)
		}
		idx.InRelease = clear
		det, err := signer.DetachedSign(release)
		if err != nil {
			return nil, fmt.Errorf("detach sign: %w", err)
		}
		idx.ReleaseGpg = det
	}
	return idx, nil
}

// generatePackagesIndex emits the Packages file body for one (component, arch).
// The result is always non-nil (an empty byte slice if there are no rows).
func generatePackagesIndex(rows []PackageRow) []byte {
	var buf bytes.Buffer
	for _, r := range rows {
		stanza := strings.TrimRight(r.RawControl, "\n")
		buf.WriteString(stanza)
		buf.WriteByte('\n')
		writeField(&buf, "Filename", r.PoolPath)
		writeFieldInt(&buf, "Size", r.Size)
		writeField(&buf, "MD5sum", r.MD5sum)
		writeField(&buf, "SHA1", r.SHA1)
		writeField(&buf, "SHA256", r.SHA256)
		if r.DescriptionMD5 != "" {
			writeField(&buf, "Description-md5", r.DescriptionMD5)
		}
		buf.WriteByte('\n')
	}
	out := buf.Bytes()
	if out == nil {
		out = []byte{}
	}
	return out
}

// fileEntry is one index file's path and bytes, used for Release checksums.
type fileEntry struct {
	path string
	data []byte
}

func (e fileEntry) md5() string  { sum := md5.Sum(e.data); return hex.EncodeToString(sum[:]) }
func (e fileEntry) sha1() string { sum := sha1.Sum(e.data); return hex.EncodeToString(sum[:]) }
func (e fileEntry) sha256() string {
	sum := sha256.Sum256(e.data)
	return hex.EncodeToString(sum[:])
}

func generateRelease(s *Suite, components, arches []string, idx *Indices) ([]byte, error) {
	var entries []fileEntry
	for path, data := range idx.Packages {
		entries = append(entries, fileEntry{path: path, data: data})
	}
	for path, data := range idx.PackagesGz {
		entries = append(entries, fileEntry{path: path, data: data})
	}
	for path, data := range idx.PackagesXz {
		entries = append(entries, fileEntry{path: path, data: data})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	var buf bytes.Buffer
	if s.Origin != "" {
		writeField(&buf, "Origin", s.Origin)
	}
	if s.Label != "" {
		writeField(&buf, "Label", s.Label)
	}
	writeField(&buf, "Suite", s.Suite)
	codename := s.Codename
	if codename == "" {
		codename = s.Suite
	}
	writeField(&buf, "Codename", codename)
	writeField(&buf, "Date", time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 MST"))
	if s.Description != "" {
		writeField(&buf, "Description", s.Description)
	}
	if len(arches) > 0 {
		writeField(&buf, "Architectures", strings.Join(arches, " "))
	}
	if len(components) > 0 {
		writeField(&buf, "Components", strings.Join(components, " "))
	}

	writeChecksumBlock(&buf, "MD5Sum", entries, func(e fileEntry) string { return e.md5() })
	writeChecksumBlock(&buf, "SHA1", entries, func(e fileEntry) string { return e.sha1() })
	writeChecksumBlock(&buf, "SHA256", entries, func(e fileEntry) string { return e.sha256() })
	return buf.Bytes(), nil
}

func writeChecksumBlock(buf *bytes.Buffer, name string, entries []fileEntry, hashFn func(fileEntry) string) {
	buf.WriteString(name + ":\n")
	for _, e := range entries {
		fmt.Fprintf(buf, " %s %16d %s\n", hashFn(e), len(e.data), e.path)
	}
}

func writeField(buf *bytes.Buffer, key, val string) {
	if val == "" {
		return
	}
	fmt.Fprintf(buf, "%s: %s\n", key, val)
}

func writeFieldInt(buf *bytes.Buffer, key string, val int64) {
	fmt.Fprintf(buf, "%s: %d\n", key, val)
}

func gzipBytes(data []byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(data)
	_ = gz.Close()
	return buf.Bytes()
}

func xzBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	xw, err := xz.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err := xw.Write(data); err != nil {
		_ = xw.Close()
		return nil, err
	}
	if err := xw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func dedupSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
