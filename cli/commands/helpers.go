package commands

import (
	"os"

	"urapt/cli/interact"
)

// resolvePassword returns the password from a flag, the URAPT_PASSWORD env
// variable, or an interactive prompt (in that order).
func (r *Root) resolvePassword(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if v := os.Getenv("URAPT_PASSWORD"); v != "" {
		return v, nil
	}
	return interact.ReadPassword("Password: ")
}
