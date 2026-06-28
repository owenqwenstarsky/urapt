// Package output provides small formatting helpers for CLI commands.
package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// JSON prints v as indented JSON.
func JSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// Printf is a thin wrapper around fmt.Printf.
func Printf(format string, args ...any) { fmt.Printf(format, args...) }

// Println prints a line.
func Println(args ...any) { fmt.Println(args...) }

// Errorf prints to stderr.
func Errorf(format string, args ...any) { fmt.Fprintf(os.Stderr, format, args...) }
