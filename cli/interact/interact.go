// Package interact provides simple interactive prompts (passwords, confirms).
package interact

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// ReadPassword prompts for a password with input hidden.
func ReadPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ReadLine prompts and reads a single line of input.
func ReadLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	var s string
	if _, err := fmt.Fscanln(os.Stdin, &s); err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

// Confirm prompts a yes/no question, returning the boolean answer.
func Confirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	var s string
	fmt.Fscanln(os.Stdin, &s)
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes"
}
