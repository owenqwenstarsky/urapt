package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"urapt/cli/output"
	"urapt/shared/apiclient"
	"urapt/shared/deb"
	"urapt/shared/models"
)

func (r *Root) pushCmd() *cobra.Command {
	var archOverride string
	cmd := &cobra.Command{
		Use:   "push <repo> <distro> <component> <file.deb>",
		Short: "Upload a .deb package to a repository",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, dist, component, file := args[0], args[1], args[2], args[3]

			// Local pre-validation for early, clear errors.
			inspected, err := deb.Inspect(file)
			if err != nil {
				return fmt.Errorf("invalid .deb: %w", err)
			}
			ctrl := inspected.Control
			pkgName, ver, arch := ctrl.Get("Package"), ctrl.Get("Version"), ctrl.Get("Architecture")
			if archOverride != "" {
				arch = archOverride
			}
			if r.flagJSON {
				// no-op: keep flag accepted
			} else {
				fmt.Printf("Pushing %s_%s_%s (%d bytes)\n", pkgName, ver, arch, inspected.Size)
			}

			c, err := r.client()
			if err != nil {
				return err
			}
			pkg, err := c.PushPackage(repo, dist, component, file)
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(pkg)
			} else {
				fmt.Printf("Pushed %s_%s_%s to %s/%s/%s\n", pkg.Name, pkg.Version, pkg.Architecture, repo, dist, component)
				fmt.Printf("  id:       %s\n", pkg.ID)
				fmt.Printf("  pool:     %s\n", pkg.PoolPath)
				fmt.Printf("  sha256:   %s\n", pkg.SHA256)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&archOverride, "arch", "", "override architecture (rarely needed)")
	return cmd
}

func (r *Root) lsCmd() *cobra.Command {
	var component, arch, name, query string
	cmd := &cobra.Command{
		Use:   "ls <repo> <distro>",
		Short: "List packages in a repository/distribution",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			resp, err := c.ListPackages(args[0], args[1], map[string]string{
				"component": component, "arch": arch, "name": name, "q": query,
			})
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(resp)
				return nil
			}
			if len(resp.Items) == 0 {
				fmt.Println("No packages.")
				return nil
			}
			fmt.Printf("%-38s  %-20s  %-12s  %-10s  %s\n", "ID", "NAME", "VERSION", "ARCH", "SIZE")
			for _, p := range resp.Items {
				fmt.Printf("%-38s  %-20s  %-12s  %-10s  %d\n", shortID(p.ID), p.Name, p.Version, p.Architecture, p.Size)
			}
			fmt.Printf("\n%d package(s)\n", resp.Total)
			return nil
		},
	}
	cmd.Flags().StringVar(&component, "component", "", "filter by component")
	cmd.Flags().StringVar(&arch, "arch", "", "filter by architecture")
	cmd.Flags().StringVar(&name, "name", "", "filter by exact name")
	cmd.Flags().StringVarP(&query, "query", "q", "", "substring search on name/description")
	return cmd
}

func (r *Root) showCmd() *cobra.Command {
	var dist string
	cmd := &cobra.Command{
		Use:   "show <repo> <id|name[@version][:arch]>",
		Short: "Show package metadata",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			id, err := r.resolvePackageID(c, args[0], dist, args[1])
			if err != nil {
				return err
			}
			p, err := c.GetPackage(args[0], id)
			if err != nil {
				return err
			}
			if r.flagJSON {
				output.JSON(p)
				return nil
			}
			printPackage(p)
			return nil
		},
	}
	cmd.Flags().StringVar(&dist, "dist", "", "distribution (required for name-based specs)")
	return cmd
}

func (r *Root) pullCmd() *cobra.Command {
	var dist, outFile string
	cmd := &cobra.Command{
		Use:   "pull <repo> <id|name[@version][:arch]>",
		Short: "Download a package's .deb file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			id, err := r.resolvePackageID(c, args[0], dist, args[1])
			if err != nil {
				return err
			}
			pkg, err := c.GetPackage(args[0], id)
			if err != nil {
				return err
			}
			out := outFile
			if out == "" {
				out = baseName(pkg.PoolPath)
			}
			if err := c.DownloadPackage(args[0], id, out); err != nil {
				return err
			}
			if outFile != "-" && !r.flagJSON {
				fmt.Printf("Downloaded %s\n", out)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dist, "dist", "", "distribution (required for name-based specs)")
	cmd.Flags().StringVarP(&outFile, "out", "o", "", "output file (default: original filename; - for stdout)")
	return cmd
}

func (r *Root) rmCmd() *cobra.Command {
	var dist string
	cmd := &cobra.Command{
		Use:   "rm <repo> <id|name[@version][:arch]>",
		Short: "Delete a package",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := r.client()
			if err != nil {
				return err
			}
			id, err := r.resolvePackageID(c, args[0], dist, args[1])
			if err != nil {
				return err
			}
			if err := c.DeletePackage(args[0], id); err != nil {
				return err
			}
			fmt.Printf("Deleted package %s\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&dist, "dist", "", "distribution (required for name-based specs)")
	return cmd
}

// resolvePackageID resolves a spec ("id" or "name[@version][:arch]") to a
// package id. UUID specs are returned as-is. Name specs require --dist.
func (r *Root) resolvePackageID(c *apiclient.Client, repo, dist, spec string) (string, error) {
	id, name, version, arch := apiclient.ParsePackageSpec(spec)
	if id != "" {
		return id, nil
	}
	if dist == "" {
		return "", fmt.Errorf("name-based spec %q requires --dist", spec)
	}
	filters := map[string]string{"name": name}
	resp, err := c.ListPackages(repo, dist, filters)
	if err != nil {
		return "", err
	}
	for _, p := range resp.Items {
		if version != "" && p.Version != version {
			continue
		}
		if arch != "" && p.Architecture != arch {
			continue
		}
		return p.ID, nil
	}
	return "", fmt.Errorf("no package matching %s in %s/%s", spec, repo, dist)
}

// printPackage prints a package's metadata in a readable key: value form.
func printPackage(p *models.Package) {
	fmt.Printf("id:           %s\n", p.ID)
	fmt.Printf("name:         %s\n", p.Name)
	fmt.Printf("version:      %s\n", p.Version)
	fmt.Printf("architecture: %s\n", p.Architecture)
	fmt.Printf("source:       %s\n", p.Source)
	fmt.Printf("maintainer:   %s\n", p.Maintainer)
	fmt.Printf("section:      %s\n", p.Section)
	fmt.Printf("priority:     %s\n", p.Priority)
	fmt.Printf("homepage:     %s\n", p.Homepage)
	fmt.Printf("depends:      %s\n", p.Depends)
	fmt.Printf("description:  %s\n", p.Description)
	fmt.Printf("pool_path:    %s\n", p.PoolPath)
	fmt.Printf("size:         %d\n", p.Size)
	fmt.Printf("sha256:       %s\n", p.SHA256)
	fmt.Printf("created_at:   %s\n", p.CreatedAt)
}

// shortID returns the first 8 chars of a UUID for compact display.
func shortID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

// baseName returns the last path segment.
func baseName(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}
