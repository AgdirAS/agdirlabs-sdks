// Command gendocs writes the CLI's Markdown command reference to cli/docs.
//
//	go run ./internal/tools/gendocs
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AgdirAS/agdirlabs-sdks/cli/internal/app"
	"github.com/spf13/cobra/doc"
)

func main() {
	dir := "docs"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(err)
	}
	old, _ := filepath.Glob(filepath.Join(dir, "agdir-email*.md"))
	for _, f := range old {
		_ = os.Remove(f)
	}
	root := app.New().RootCommand()
	root.DisableAutoGenTag = true
	link := func(name string) string { return strings.ToLower(name) }
	if err := doc.GenMarkdownTreeCustom(root, dir, func(string) string { return "" }, link); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
