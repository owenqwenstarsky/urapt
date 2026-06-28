package commands

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"urapt/shared/models"
)

func (r *Root) aptConfigCmd() *cobra.Command {
	var component, signedBy string
	cmd := &cobra.Command{
		Use:   "apt-config <repo> <distro>",
		Short: "Print apt client configuration (sources.list, key, and auth) for a repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoName, dist := args[0], args[1]
			c, err := r.client()
			if err != nil {
				return err
			}
			repo, err := c.GetRepository(repoName)
			if err != nil {
				return err
			}
			comps, err := c.ListComponents(repoName, dist)
			if err != nil {
				return err
			}
			arches, err := c.ListArchitectures(repoName, dist)
			if err != nil {
				return err
			}

			server, err := r.server()
			if err != nil {
				return err
			}
			if signedBy == "" {
				signedBy = "/usr/share/keyrings/urapt-" + repoName + ".gpg"
			}

			compList := component
			if compList == "" {
				var names []string
				for _, comp := range comps {
					names = append(names, comp.Name)
				}
				compList = strings.Join(names, " ")
			}
			if compList == "" {
				compList = "main"
			}

			var archStr string
			if len(arches) > 0 {
				var names []string
				for _, a := range arches {
					names = append(names, a.Name)
				}
				archStr = strings.Join(names, ",")
			}

			fmt.Println("# 1. Install the repository signing key:")
			fmt.Printf("curl -fsSL %s/api/v1/server/pubkey | sudo gpg --dearmor -o %s\n\n", server, signedBy)

			fmt.Println("# 2. Add the repository to apt:")
			line := fmt.Sprintf("deb [signed-by=%s]", signedBy)
			if archStr != "" {
				line = fmt.Sprintf("deb [arch=%s signed-by=%s]", archStr, signedBy)
			}
			line += fmt.Sprintf(" %s/apt/%s/ %s %s", server, repoName, dist, compList)
			fmt.Printf("echo '%s' | sudo tee /etc/apt/sources.list.d/%s.list\n\n", line, repoName)

			fmt.Println("# 3. Update apt:")
			fmt.Println("sudo apt update")
			fmt.Println()

			if repo.Visibility == models.VisibilityPrivate {
				host := hostOf(server)
				fmt.Println("# 4. This repository is private. Configure apt credentials:")
				fmt.Printf("sudo tee /etc/apt/auth.conf.d/%s.conf <<EOF\n", repoName)
				fmt.Printf("machine %s\n", host)
				fmt.Printf("login %s\n", r.cfg.Default.User)
				tok, _ := r.token()
				fmt.Printf("password %s\n", tok)
				fmt.Println("EOF")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&component, "component", "", "components to enable (default: all in distribution)")
	cmd.Flags().StringVar(&signedBy, "signed-by", "", "path for the dearmored keyring")
	return cmd
}

// hostOf extracts the host (and port) from a server URL for auth.conf.
func hostOf(server string) string {
	u, err := url.Parse(server)
	if err != nil {
		return server
	}
	return u.Host
}
