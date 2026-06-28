package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"urapt/cli/output"
)

func (r *Root) distroCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "distro",
		Short: "Manage distributions (suites) within a repository",
	}
	cmd.AddCommand(r.distroCreateCmd(), r.distroListCmd(), r.distroDeleteCmd())
	return cmd
}

func (r *Root) distroCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <repo> <distro>",
		Short: "Add a distribution to a repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			d, err := c.CreateDistribution(args[0], args[1])
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(d)
			} else {
				fmt.Printf("Created distribution %s in %s\n", d.Name, args[0])
			}
			return nil
		},
	}
}

func (r *Root) distroListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <repo>",
		Short: "List distributions in a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			dists, err := c.ListDistributions(args[0])
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(dists)
				return nil
			}
			for _, d := range dists {
				fmt.Println(d.Name)
			}
			return nil
		},
	}
}

func (r *Root) distroDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <repo> <distro>",
		Short: "Delete a distribution and its packages",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			if err := c.DeleteDistribution(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Deleted distribution %s in %s\n", args[1], args[0])
			return nil
		},
	}
}

func (r *Root) componentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component",
		Short: "Manage components within a distribution",
	}
	cmd.AddCommand(r.componentCreateCmd(), r.componentListCmd(), r.componentDeleteCmd())
	return cmd
}

func (r *Root) componentCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <repo> <distro> <component>",
		Short: "Add a component to a distribution",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			comp, err := c.CreateComponent(args[0], args[1], args[2])
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(comp)
			} else {
				fmt.Printf("Created component %s in %s/%s\n", comp.Name, args[0], args[1])
			}
			return nil
		},
	}
}

func (r *Root) componentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <repo> <distro>",
		Short: "List components in a distribution",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			comps, err := c.ListComponents(args[0], args[1])
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(comps)
				return nil
			}
			for _, comp := range comps {
				fmt.Println(comp.Name)
			}
			return nil
		},
	}
}

func (r *Root) componentDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <repo> <distro> <component>",
		Short: "Delete a component",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			if err := c.DeleteComponent(args[0], args[1], args[2]); err != nil {
				return err
			}
			fmt.Printf("Deleted component %s in %s/%s\n", args[2], args[0], args[1])
			return nil
		},
	}
}

func (r *Root) archCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "arch",
		Short: "Manage architectures within a distribution",
	}
	cmd.AddCommand(r.archAddCmd(), r.archListCmd(), r.archRemoveCmd())
	return cmd
}

func (r *Root) archAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <repo> <distro> <arch>",
		Short: "Add an architecture to a distribution",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			a, err := c.CreateArchitecture(args[0], args[1], args[2])
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(a)
			} else {
				fmt.Printf("Added architecture %s to %s/%s\n", a.Name, args[0], args[1])
			}
			return nil
		},
	}
}

func (r *Root) archListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <repo> <distro>",
		Short: "List architectures in a distribution",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			arches, err := c.ListArchitectures(args[0], args[1])
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(arches)
				return nil
			}
			for _, a := range arches {
				fmt.Println(a.Name)
			}
			return nil
		},
	}
}

func (r *Root) archRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <repo> <distro> <arch>",
		Short: "Remove an architecture from a distribution",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			if err := c.DeleteArchitecture(args[0], args[1], args[2]); err != nil {
				return err
			}
			fmt.Printf("Removed architecture %s from %s/%s\n", args[2], args[0], args[1])
			return nil
		},
	}
}
