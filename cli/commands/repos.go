package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"urapt/cli/output"
)

func (r *Root) repoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage repositories",
	}
	cmd.AddCommand(
		r.repoCreateCmd(),
		r.repoListCmd(),
		r.repoInfoCmd(),
		r.repoSetVisibilityCmd(),
		r.repoDeleteCmd(),
		r.repoPubkeyCmd(),
		r.repoMembersCmd(),
	)
	return cmd
}

func (r *Root) repoCreateCmd() *cobra.Command {
	var visibility, description string
	var public, private bool
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			vis := visibility
			if public {
				vis = "public"
			} else if private {
				vis = "private"
			}
			if vis == "" {
				vis = "private"
			}
			c, err := r.client()
			if err != nil {
				return err
			}
			repo, err := c.CreateRepository(args[0], vis, description)
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(repo)
			} else {
				fmt.Printf("Created repository %s (%s)\n", repo.Name, repo.Visibility)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&visibility, "visibility", "", "public or private")
	cmd.Flags().BoolVar(&public, "public", false, "shorthand for --visibility=public")
	cmd.Flags().BoolVar(&private, "private", false, "shorthand for --visibility=private")
	cmd.Flags().StringVarP(&description, "description", "d", "", "repository description")
	return cmd
}

func (r *Root) repoListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List repositories visible to you",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			repos, err := c.ListRepositories()
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(repos)
				return nil
			}
			if len(repos) == 0 {
				fmt.Println("No repositories.")
				return nil
			}
			fmt.Printf("%-20s  %-10s  %s\n", "NAME", "VISIBILITY", "DESCRIPTION")
			for _, repo := range repos {
				fmt.Printf("%-20s  %-10s  %s\n", repo.Name, repo.Visibility, repo.Description)
			}
			return nil
		},
	}
}

func (r *Root) repoInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show details of a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			repo, err := c.GetRepository(args[0])
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(repo)
				return nil
			}
			fmt.Printf("name:        %s\n", repo.Name)
			fmt.Printf("visibility:  %s\n", repo.Visibility)
			fmt.Printf("description: %s\n", repo.Description)
			fmt.Printf("created:     %s\n", repo.CreatedAt)
			return nil
		},
	}
}

func (r *Root) repoSetVisibilityCmd() *cobra.Command {
	var public, private bool
	cmd := &cobra.Command{
		Use:   "set-visibility <name>",
		Short: "Change a repository's visibility",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			vis := ""
			if public {
				vis = "public"
			} else if private {
				vis = "private"
			}
			if vis == "" {
				return fmt.Errorf("pass --public or --private")
			}
			c, err := r.client()
			if err != nil {
				return err
			}
			repo, err := c.UpdateRepository(args[0], nil, &vis, nil)
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(repo)
			} else {
				fmt.Printf("Updated %s: visibility=%s\n", repo.Name, repo.Visibility)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&public, "public", false, "make public")
	cmd.Flags().BoolVar(&private, "private", false, "make private")
	return cmd
}

func (r *Root) repoDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a repository and all its packages",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			if err := c.DeleteRepository(args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted repository %s\n", args[0])
			return nil
		},
	}
}

func (r *Root) repoPubkeyCmd() *cobra.Command {
	var outFile string
	cmd := &cobra.Command{
		Use:   "pubkey <name>",
		Short: "Print the server's armored public key for a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			pub, err := c.RepositoryPubkey(args[0])
			if err != nil {
				return err
			}
			if outFile != "" {
				return os.WriteFile(outFile, []byte(pub), 0o644)
			}
			fmt.Print(pub)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outFile, "out", "o", "", "write to file instead of stdout")
	return cmd
}

func (r *Root) repoMembersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "Manage repository members",
	}
	cmd.AddCommand(
		r.repoMembersListCmd(),
		r.repoMembersAddCmd(),
		r.repoMembersUpdateCmd(),
		r.repoMembersRemoveCmd(),
	)
	return cmd
}

func (r *Root) repoMembersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <repo>",
		Short: "List members of a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			members, err := c.ListMembers(args[0])
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(members)
				return nil
			}
			fmt.Printf("%-20s  %s\n", "USER", "ACCESS")
			for _, m := range members {
				fmt.Printf("%-20s  %s\n", m.User.Username, m.Access)
			}
			return nil
		},
	}
}

func (r *Root) repoMembersAddCmd() *cobra.Command {
	var access string
	cmd := &cobra.Command{
		Use:   "add <repo> <username>",
		Short: "Grant a user access to a repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			m, err := c.AddMember(args[0], args[1], access)
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(m)
			} else {
				fmt.Printf("Granted %s access=%s on %s\n", m.User.Username, m.Access, args[0])
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&access, "access", "a", "read", "read, write, read-write, or admin")
	return cmd
}

func (r *Root) repoMembersUpdateCmd() *cobra.Command {
	var access string
	cmd := &cobra.Command{
		Use:   "update <repo> <username>",
		Short: "Change a member's access level",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			m, err := c.UpdateMember(args[0], args[1], access)
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(m)
			} else {
				fmt.Printf("Updated %s access=%s on %s\n", m.User.Username, m.Access, args[0])
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&access, "access", "a", "", "read, write, read-write, or admin")
	return cmd
}

func (r *Root) repoMembersRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <repo> <username>",
		Short: "Revoke a user's access to a repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			if err := c.RemoveMember(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Removed %s from %s\n", args[1], args[0])
			return nil
		},
	}
}
