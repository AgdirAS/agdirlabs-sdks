package app

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
	"github.com/spf13/cobra"
)

func (a *App) newDomainsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "domains",
		Short: "Add, verify and check sending domains",
		Long: `Manage sending domains. Commands that take a <domain> accept its ID or its name.

Typical setup:
  agdir-email domains add acme.com       # prints the DNS records to publish
  agdir-email domains check acme.com     # checks them in live DNS
  agdir-email domains verify acme.com    # asks Agdir Email to verify now`,
	}
	c.AddCommand(a.newDomainsAddCmd(), a.newDomainsListCmd(), a.newDomainsGetCmd(),
		a.newDomainsVerifyCmd(), a.newDomainsDeleteCmd(), a.newDomainsCheckCmd())
	return c
}

// resolveDomainID turns a domain name into its ID (IDs are returned unchanged).
func resolveDomainID(ctx context.Context, client *agdiremail.Client, arg string) (string, error) {
	if !strings.Contains(arg, ".") {
		return arg, nil
	}
	name := strings.TrimSuffix(strings.ToLower(arg), ".")
	for d, err := range client.Domains.ListAll(ctx, &agdiremail.ListParams{Limit: 100}) {
		if err != nil {
			return "", err
		}
		if strings.EqualFold(d.Name, name) {
			return d.ID, nil
		}
	}
	return "", failf("no domain named %s; add it with \"agdir-email domains add %s\"", name, name)
}

func recordRows(records []agdiremail.DNSRecord, withStatus bool) [][]string {
	rows := make([][]string, len(records))
	for i, r := range records {
		prio := ""
		if r.Priority != 0 {
			prio = strconv.Itoa(r.Priority)
		}
		rows[i] = []string{r.Purpose, r.Type, r.Name, orDash(prio)}
		if withStatus {
			rows[i] = append(rows[i], string(r.Status))
		}
		rows[i] = append(rows[i], r.Value)
	}
	return rows
}

// The value goes last: DKIM keys are long.
func recordHeaders(withStatus bool) []string {
	if withStatus {
		return []string{"PURPOSE", "TYPE", "NAME", "PRIORITY", "STATUS", "VALUE"}
	}
	return []string{"PURPOSE", "TYPE", "NAME", "PRIORITY", "VALUE"}
}

func (a *App) newDomainsAddCmd() *cobra.Command {
	var region string
	c := &cobra.Command{
		Use:     "add <name>",
		Aliases: []string{"create"},
		Short:   "Add a sending domain and print its DNS records",
		Long: `Add a sending domain and print the DNS records to publish at your DNS provider.
Record names are relative to your registrable domain (e.g. "send" means send.acme.com).
Then run "agdir-email domains check <name>" to see which records are live.`,
		Example: "  agdir-email domains add acme.com\n  agdir-email domains add mail.acme.com --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			d, err := client.Domains.Create(cmd.Context(), &agdiremail.CreateDomainRequest{Name: args[0], Region: region})
			if err != nil {
				return err
			}
			if a.opts.json {
				return a.printJSON(d)
			}
			if a.opts.quiet {
				a.printIDs(d.ID)
				return nil
			}
			a.printf("Added %s (%s, status %s).\n\nPublish these DNS records at your DNS provider:\n\n", d.Name, d.ID, d.Status)
			table(a.Stdout, recordHeaders(false), recordRows(d.Records, false))
			a.printf("\nDMARC is recommended. Names are relative to your registrable domain.\n")
			a.printf("Next: agdir-email domains check %s   (then: agdir-email domains verify %s)\n", d.Name, d.Name)
			return nil
		},
	}
	c.Flags().StringVar(&region, "region", "", "sending region (default: the API's, eu-north-1)")
	return c
}

func (a *App) newDomainsListCmd() *cobra.Command {
	var lf listFlags
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List domains",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			page, err := fetchList(cmd.Context(), lf, client.Domains.List, client.Domains.ListAll)
			if err != nil {
				return err
			}
			if a.opts.json {
				return a.printJSON(page)
			}
			if a.opts.quiet {
				for _, d := range page.Data {
					a.printIDs(d.ID)
				}
				return nil
			}
			if len(page.Data) == 0 {
				a.printf("No domains yet. Add one with \"agdir-email domains add <name>\".\n")
				return nil
			}
			rows := make([][]string, len(page.Data))
			for i, d := range page.Data {
				rows[i] = []string{d.ID, d.Name, string(d.Status), d.Region, formatTime(d.CreatedAt)}
			}
			table(a.Stdout, []string{"ID", "NAME", "STATUS", "REGION", "CREATED"}, rows)
			a.printPageFooter(cmd, lf, page.NextCursor)
			return nil
		},
	}
	lf.register(c)
	return c
}

func (a *App) newDomainsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <domain>",
		Short: "Show a domain and the status of its DNS records",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			id, err := resolveDomainID(cmd.Context(), client, args[0])
			if err != nil {
				return err
			}
			d, err := client.Domains.Get(cmd.Context(), id)
			if err != nil {
				return err
			}
			if a.opts.json {
				return a.printJSON(d)
			}
			if a.opts.quiet {
				a.printIDs(d.ID)
				return nil
			}
			fields(a.Stdout, [][2]string{
				{"ID", d.ID},
				{"Name", d.Name},
				{"Status", string(d.Status)},
				{"Region", d.Region},
				{"Created", formatTime(d.CreatedAt)},
				{"Open tracking", strconv.FormatBool(d.OpenTracking)},
				{"Click tracking", strconv.FormatBool(d.ClickTracking)},
			})
			if len(d.Records) > 0 {
				a.printf("\n")
				table(a.Stdout, recordHeaders(true), recordRows(d.Records, true))
			}
			return nil
		},
	}
}

func (a *App) newDomainsVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <domain>",
		Short: "Ask Agdir Email to check the domain's DNS records now",
		Long: `Start verification of a domain now instead of waiting for the periodic check.
Verification runs in the background: follow it with "agdir-email domains get <domain>".
To see what is published in DNS right now, use "agdir-email domains check <domain>".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			id, err := resolveDomainID(cmd.Context(), client, args[0])
			if err != nil {
				return err
			}
			res, err := client.Domains.Verify(cmd.Context(), id)
			if err != nil {
				return err
			}
			switch {
			case a.opts.json:
				return a.printJSON(map[string]any{"id": res.ID, "verification_started": true})
			case a.opts.quiet:
				a.printIDs(res.ID)
			default:
				a.printf("Verification started for %s. Follow it with: agdir-email domains get %s\n", args[0], args[0])
			}
			return nil
		},
	}
}

func (a *App) newDomainsDeleteCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:     "delete <domain>",
		Aliases: []string{"rm"},
		Short:   "Delete a domain",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.confirm(yes, fmt.Sprintf("Delete domain %s? You will no longer be able to send from it.", args[0])); err != nil {
				return err
			}
			client, err := a.client()
			if err != nil {
				return err
			}
			id, err := resolveDomainID(cmd.Context(), client, args[0])
			if err != nil {
				return err
			}
			res, err := client.Domains.Delete(cmd.Context(), id)
			if err != nil {
				return err
			}
			switch {
			case a.opts.json:
				return a.printJSON(res)
			case a.opts.quiet:
				a.printIDs(res.ID)
			default:
				a.printf("Deleted domain %s.\n", args[0])
			}
			return nil
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask for confirmation (required when not interactive)")
	return c
}

// confirm asks a yes/no question on an interactive terminal; non-interactive runs need --yes.
func (a *App) confirm(yes bool, question string) error {
	if yes {
		return nil
	}
	if !a.stdinIsTerminal() {
		return usageErrorf("refusing to delete without confirmation: pass --yes")
	}
	fmt.Fprintf(a.Stderr, "%s [y/N] ", question)
	answer, err := readLine(a.Stdin)
	if err != nil {
		return err
	}
	if a := strings.ToLower(answer); a != "y" && a != "yes" {
		return failf("aborted")
	}
	return nil
}

func (a *App) newDomainsCheckCmd() *cobra.Command {
	var nameserver string
	c := &cobra.Command{
		Use:   "check <domain>",
		Short: "Check the domain's DNS records in live DNS",
		Long: `Look up each DNS record the domain needs and compare it with the expected value,
so you can see exactly what is missing or wrong before verifying. This queries DNS
from your machine (use --nameserver to ask a specific server and skip local caches).

DKIM and SPF records are required; DMARC is recommended. Exits with code 1 if a required
record is missing or wrong.`,
		Example: "  agdir-email domains check acme.com\n  agdir-email domains check acme.com --nameserver 1.1.1.1",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			id, err := resolveDomainID(cmd.Context(), client, args[0])
			if err != nil {
				return err
			}
			d, err := client.Domains.Get(cmd.Context(), id)
			if err != nil {
				return err
			}
			var resolver Resolver = net.DefaultResolver
			if a.Resolver != nil {
				resolver = a.Resolver
			}
			if nameserver != "" {
				resolver = nameserverResolver(nameserver)
			}
			checks := checkRecords(cmd.Context(), resolver, d.Name, d.Records)
			failed := 0
			for _, c := range checks {
				if c.Required && !c.OK {
					failed++
				}
			}

			if a.opts.json {
				if err := a.printJSON(map[string]any{
					"id": d.ID, "name": d.Name, "status": d.Status, "ok": failed == 0, "records": checks,
				}); err != nil {
					return err
				}
			} else if !a.opts.quiet {
				a.printf("%s (status: %s)\n\n", d.Name, d.Status)
				rows := make([][]string, len(checks))
				for i, c := range checks {
					mark, purpose := "✓", c.Purpose
					if !c.OK {
						mark = "✗"
					}
					if !c.Required {
						purpose += " (recommended)"
						if !c.OK {
							mark = "!"
						}
					}
					result := "ok"
					switch {
					case c.OK && c.Problem != "":
						result = "ok — " + c.Problem
					case !c.OK:
						result = c.Problem
						if len(c.Found) > 0 {
							result += "; found: " + truncate(strings.Join(c.Found, " | "), 60)
						}
						result += "; expected: " + truncate(c.Expected, 60)
					}
					rows[i] = []string{mark, purpose, c.Type, c.FQDN, result}
				}
				table(a.Stdout, []string{"", "RECORD", "TYPE", "NAME", "RESULT"}, rows)
				a.printf("\n")
				if failed == 0 {
					if d.Status == agdiremail.DomainVerified {
						a.printf("All required records are published and the domain is verified.\n")
					} else {
						a.printf("All required records are published. Run \"agdir-email domains verify %s\" to verify now.\n", d.Name)
					}
				} else {
					a.printf("%d required record(s) missing or wrong. Fix them at your DNS provider (see \"agdir-email domains get %s\" for the full values);\nDNS changes can take a few minutes, sometimes hours, to propagate.\n", failed, d.Name)
				}
			}
			if failed > 0 {
				return silentFailure(fmt.Errorf("%d required DNS records missing or wrong", failed))
			}
			return nil
		},
	}
	c.Flags().StringVar(&nameserver, "nameserver", "", "DNS server to query, e.g. 1.1.1.1 or 8.8.8.8:53 (default: the system resolver)")
	return c
}
