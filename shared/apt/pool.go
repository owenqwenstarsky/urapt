package apt

import "strings"

// PoolPath computes the conventional Debian pool path for a package:
// pool/<component>/<letter>/<src>/<filename>, where <src> is the source package
// name (or the binary package name if absent) and <letter> follows the Debian
// "libX" convention.
func PoolPath(component, source, packageName, filename string) string {
	src := source
	if src == "" {
		src = packageName
	}
	letter := poolLetter(src)
	return strings.Join([]string{"pool", component, letter, src, filename}, "/")
}

// poolLetter returns the pool subdirectory prefix for a source name: "lib" +
// next char for names starting with "lib", otherwise the first character.
func poolLetter(src string) string {
	if len(src) >= 4 && strings.HasPrefix(src, "lib") {
		return "lib" + string(src[3])
	}
	if src == "" {
		return "0"
	}
	return string(src[0])
}
