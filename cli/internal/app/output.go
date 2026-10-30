package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
	"github.com/spf13/cobra"
)

// printJSON writes v as indented JSON to stdout.
func (a *App) printJSON(v any) error {
	enc := json.NewEncoder(a.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printf writes human output unless --quiet or --json is set.
func (a *App) printf(format string, args ...any) {
	if a.opts.quiet || a.opts.json {
		return
	}
	fmt.Fprintf(a.Stdout, format, args...)
}

// printIDs writes one ID per line (the --quiet output).
func (a *App) printIDs(ids ...string) {
	for _, id := range ids {
		fmt.Fprintln(a.Stdout, id)
	}
}

// table writes rows aligned in columns under upper-case headers.
func table(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	tw.Flush()
}

// fields writes "Label:  value" lines, skipping empty values.
func fields(w io.Writer, pairs [][2]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, p := range pairs {
		if p[1] != "" {
			fmt.Fprintf(tw, "%s:\t%s\n", p[0], p[1])
		}
	}
	tw.Flush()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05 MST")
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

// truncate shortens s to n runes with an ellipsis, on one line.
func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// listFlags are the pagination flags of list commands.
type listFlags struct {
	limit int
	after string
}

func (f *listFlags) register(c *cobra.Command) {
	c.Flags().IntVar(&f.limit, "limit", 0, "fetch one page of at most this many items (1-100); default: all items")
	c.Flags().StringVar(&f.after, "after", "", "cursor: list items after this ID (the previous page's next_cursor)")
}

// fetchList returns one page when --limit/--after is given, else every item.
func fetchList[T any](ctx context.Context, f listFlags,
	page func(context.Context, *agdiremail.ListParams) (*agdiremail.Page[T], error),
	all func(context.Context, *agdiremail.ListParams) iter.Seq2[T, error],
) (*agdiremail.Page[T], error) {
	if f.limit < 0 || f.limit > 100 {
		return nil, usageErrorf("--limit must be between 1 and 100")
	}
	if f.limit > 0 || f.after != "" {
		return page(ctx, &agdiremail.ListParams{Limit: f.limit, After: f.after})
	}
	out := &agdiremail.Page[T]{Data: []T{}}
	for item, err := range all(ctx, &agdiremail.ListParams{Limit: 100}) {
		if err != nil {
			return nil, err
		}
		out.Data = append(out.Data, item)
	}
	return out, nil
}

// printPageFooter tells the user how to get the next page.
func (a *App) printPageFooter(cmd *cobra.Command, f listFlags, cursor string) {
	if cursor == "" {
		return
	}
	next := cmd.CommandPath()
	for _, arg := range cmd.Flags().Args() {
		next += " " + arg
	}
	if f.limit > 0 {
		next += fmt.Sprintf(" --limit %d", f.limit)
	}
	a.printf("\nMore results: %s --after %s\n", next, cursor)
}
