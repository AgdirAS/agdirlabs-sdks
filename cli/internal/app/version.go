package app

import (
	"runtime"
	"runtime/debug"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
	"github.com/spf13/cobra"
)

// Build information, set by GoReleaser with -ldflags "-X …".
var (
	Version = "0.1.0"
	Commit  = ""
	Date    = ""
)

func buildCommit() string {
	if Commit != "" {
		return Commit
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 12 {
				return s.Value[:12]
			}
		}
	}
	return ""
}

func (a *App) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(_ *cobra.Command, _ []string) error {
			info := map[string]string{
				"version":    Version,
				"commit":     buildCommit(),
				"date":       Date,
				"sdk":        agdiremail.Version,
				"go":         runtime.Version(),
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
				"user_agent": userAgent(),
			}
			if a.opts.json {
				return a.printJSON(info)
			}
			if a.opts.quiet {
				a.printIDs(Version)
				return nil
			}
			line := "agdir-email " + Version
			if c := info["commit"]; c != "" {
				line += " (" + c
				if Date != "" {
					line += ", " + Date
				}
				line += ")"
			}
			a.printf("%s\nGo SDK %s, %s %s/%s\n", line, agdiremail.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}
