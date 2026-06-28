package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"urapt/cli/interact"
	"urapt/cli/output"
)

func (r *Root) loginCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "login [server]",
		Short: "Log in to a urapt server and save an API token",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			server := r.flagServer
			if len(args) == 1 {
				server = args[0]
			}
			if server == "" {
				return fmt.Errorf("server URL required: pass as an argument or --server")
			}
			if username == "" {
				username = r.flagUser
			}
			if username == "" {
				var err error
				username, err = interact.ReadLine("Username: ")
				if err != nil {
					return err
				}
			}
			pw, err := r.resolvePassword(password)
			if err != nil {
				return err
			}
			c, err := r.unauthClient(server)
			if err != nil {
				return err
			}
			user, token, err := c.Login(username, pw)
			if err != nil {
				return err
			}
			if err := r.saveProfile(server, user.Username, token); err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(user)
			} else {
				fmt.Printf("Logged in as %s (server: %s)\n", user.Username, server)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "password (prompts if omitted)")
	return cmd
}

func (r *Root) logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the current API token and clear the saved profile",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			if err := c.Logout(); err != nil {
				return err
			}
			if err := r.clearProfile(); err != nil {
				return err
			}
			fmt.Println("Logged out")
			return nil
		},
	}
}

func (r *Root) whoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the currently authenticated user",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			user, err := c.Me()
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(user)
			} else {
				fmt.Printf("%s (admin: %v)\n", user.Username, user.IsAdmin)
			}
			return nil
		},
	}
}

func (r *Root) registerCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "register [server]",
		Short: "Create a new account on a urapt server (first account becomes admin)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			server := r.flagServer
			if len(args) == 1 {
				server = args[0]
			}
			if server == "" {
				return fmt.Errorf("server URL required: pass as an argument or --server")
			}
			if username == "" {
				username = r.flagUser
			}
			if username == "" {
				var err error
				username, err = interact.ReadLine("Username: ")
				if err != nil {
					return err
				}
			}
			pw, err := r.resolvePassword(password)
			if err != nil {
				return err
			}
			c, err := r.unauthClient(server)
			if err != nil {
				return err
			}
			user, token, err := c.Register(username, pw)
			if err != nil {
				return err
			}
			if err := r.saveProfile(server, user.Username, token); err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(user)
			} else {
				fmt.Printf("Registered and logged in as %s (admin: %v)\n", user.Username, user.IsAdmin)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "password (prompts if omitted)")
	return cmd
}

func (r *Root) tokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage API tokens",
	}
	cmd.AddCommand(
		r.tokenCreateCmd(),
		r.tokenListCmd(),
		r.tokenRevokeCmd(),
	)
	return cmd
}

func (r *Root) tokenCreateCmd() *cobra.Command {
	var name string
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new API token",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := r.client()
			if err != nil {
				return err
			}
			t, err := client.CreateToken(name)
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(t)
			} else {
				fmt.Printf("Token created (name: %s)\n", t.Name)
				fmt.Printf("  %s\n", t.Token)
				fmt.Println("  Save this token; it will not be shown again.")
			}
			return nil
		},
	}
	c.Flags().StringVarP(&name, "name", "n", "default", "token label")
	return c
}

func (r *Root) tokenListCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "List your API tokens",
		RunE: func(_ *cobra.Command, _ []string) error {
			client, err := r.client()
			if err != nil {
				return err
			}
			tokens, err := client.ListTokens()
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(tokens)
				return nil
			}
			if len(tokens) == 0 {
				fmt.Println("No tokens.")
				return nil
			}
			fmt.Printf("%-12s  %-20s  %-10s  %s\n", "PREFIX", "NAME", "CREATED", "STATUS")
			for _, t := range tokens {
				status := "active"
				if t.RevokedAt != nil {
					status = "revoked"
				}
				fmt.Printf("%-12s  %-20s  %-10s  %s\n", t.Prefix, t.Name, t.CreatedAt[:10], status)
			}
			return nil
		},
	}
	return c
}

func (r *Root) tokenRevokeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke an API token by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client, err := r.client()
			if err != nil {
				return err
			}
			if err := client.RevokeToken(args[0]); err != nil {
				return err
			}
			fmt.Println("Token revoked")
			return nil
		},
	}
	return c
}
