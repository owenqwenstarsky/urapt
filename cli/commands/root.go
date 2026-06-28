// Package commands implements the urapt CLI command tree using cobra.
package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"urapt/cli/config"
	"urapt/shared/apiclient"
)

// Root holds the urapt CLI runtime state: the version, loaded config, and
// flag overrides. It builds and executes the cobra command tree.
type Root struct {
	version string

	cfg *config.File

	flagServer string
	flagUser   string
	flagToken  string
	flagJSON   bool

	rootCmd *cobra.Command
}

// New constructs the CLI root.
func New(version string) *Root {
	r := &Root{version: version}
	r.build()
	return r
}

// Execute runs the command tree with the given args.
func (r *Root) Execute(args []string) error {
	r.rootCmd.SetArgs(args)
	err := r.rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	return err
}

// build constructs the cobra root and attaches subcommands.
func (r *Root) build() {
	r.rootCmd = &cobra.Command{
		Use:   "urapt",
		Short: "urapt is a CLI for managing packages on a self-hosted urapt APT server",
		Long: "urapt pushes and manages Debian packages on a self-hosted urapt\n" +
			"server. Log in once, then push .deb files to your repositories.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	r.rootCmd.PersistentFlags().StringVar(&r.flagServer, "server", "", "urapt server URL (overrides config)")
	r.rootCmd.PersistentFlags().StringVar(&r.flagUser, "user", "", "username (overrides config)")
	r.rootCmd.PersistentFlags().StringVar(&r.flagToken, "token", "", "API token (overrides config)")
	r.rootCmd.PersistentFlags().BoolVar(&r.flagJSON, "json", false, "output JSON")

	r.rootCmd.AddCommand(
		r.versionCmd(),
		r.loginCmd(),
		r.logoutCmd(),
		r.whoamiCmd(),
		r.registerCmd(),
		r.tokenCmd(),
		r.repoCmd(),
		r.distroCmd(),
		r.componentCmd(),
		r.archCmd(),
		r.pushCmd(),
		r.pullCmd(),
		r.lsCmd(),
		r.showCmd(),
		r.rmCmd(),
		r.aptConfigCmd(),
	)
}

// loadConfig lazily loads the CLI config file (once).
func (r *Root) loadConfig() error {
	if r.cfg != nil {
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	r.cfg = cfg
	return nil
}

// server resolves the effective server URL (flag > env > config).
func (r *Root) server() (string, error) {
	if err := r.loadConfig(); err != nil {
		return "", err
	}
	if r.flagServer != "" {
		return r.flagServer, nil
	}
	if v := os.Getenv("URAPT_SERVER"); v != "" {
		return v, nil
	}
	if r.cfg.Default.Server != "" {
		return r.cfg.Default.Server, nil
	}
	return "", fmt.Errorf("no server configured: run `urapt login <server>` or pass --server")
}

// token resolves the effective API token (flag > env > config).
func (r *Root) token() (string, error) {
	if r.flagToken != "" {
		return r.flagToken, nil
	}
	if v := os.Getenv("URAPT_TOKEN"); v != "" {
		return v, nil
	}
	if err := r.loadConfig(); err != nil {
		return "", err
	}
	if r.cfg.Default.Token != "" {
		return r.cfg.Default.Token, nil
	}
	return "", fmt.Errorf("not logged in: run `urapt login`")
}

// client builds an authenticated client using the resolved server + token.
func (r *Root) client() (*apiclient.Client, error) {
	server, err := r.server()
	if err != nil {
		return nil, err
	}
	tok, _ := r.token()
	return apiclient.New(server, tok), nil
}

// unauthClient builds a client with only the server resolved (for login/register).
func (r *Root) unauthClient(server string) (*apiclient.Client, error) {
	if server == "" {
		var err error
		server, err = r.server()
		if err != nil {
			return nil, err
		}
	}
	return apiclient.New(server, ""), nil
}

// saveProfile persists the given server/user/token as the default profile.
func (r *Root) saveProfile(server, user, token string) error {
	if err := r.loadConfig(); err != nil {
		return err
	}
	r.cfg.Default.Server = server
	r.cfg.Default.User = user
	r.cfg.Default.Token = token
	return config.Save(r.cfg)
}

// clearProfile removes the stored token (used by logout).
func (r *Root) clearProfile() error {
	if err := r.loadConfig(); err != nil {
		return err
	}
	r.cfg.Default.Token = ""
	return config.Save(r.cfg)
}
