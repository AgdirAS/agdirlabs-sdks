package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
	"github.com/spf13/cobra"
)

func (a *App) newWebhooksCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "webhooks",
		Short: "Manage webhook endpoints and their deliveries",
		Long: `Manage webhook endpoints that receive email events, inspect what was delivered to them,
and verify signatures of captured requests.

Event types: ` + strings.Join(eventNames(), ", ") + `.`,
	}
	c.AddCommand(a.newWebhooksCreateCmd(), a.newWebhooksListCmd(), a.newWebhooksGetCmd(),
		a.newWebhooksUpdateCmd(), a.newWebhooksDeleteCmd(), a.newWebhooksRotateCmd(),
		a.newDeliveriesCmd(), a.newWebhooksVerifyCmd())
	return c
}

func eventNames() []string {
	var out []string
	for _, e := range agdiremail.WebhookEventTypes() {
		out = append(out, string(e))
	}
	return out
}

// parseEvents accepts full names ("email.delivered"), short names ("delivered") and "all".
func parseEvents(values []string) ([]agdiremail.WebhookEventType, error) {
	known := agdiremail.WebhookEventTypes()
	var out []agdiremail.WebhookEventType
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		if v == "all" || v == "*" {
			return known, nil
		}
		if !strings.HasPrefix(v, "email.") {
			v = "email." + v
		}
		e := agdiremail.WebhookEventType(v)
		if !slices.Contains(known, e) {
			return nil, usageErrorf("unknown event %q; valid events: %s, or all", v, strings.Join(eventNames(), ", "))
		}
		if !slices.Contains(out, e) {
			out = append(out, e)
		}
	}
	return out, nil
}

func joinEvents(events []agdiremail.WebhookEventType) string {
	if len(events) == len(agdiremail.WebhookEventTypes()) {
		return "all"
	}
	s := make([]string, len(events))
	for i, e := range events {
		s[i] = strings.TrimPrefix(string(e), "email.")
	}
	return strings.Join(s, ",")
}

func (a *App) newWebhooksCreateCmd() *cobra.Command {
	var endpoint string
	var events []string
	c := &cobra.Command{
		Use:   "create",
		Short: "Register a webhook endpoint",
		Example: `  agdir-email webhooks create --endpoint https://acme.com/webhooks/email --event all
  agdir-email webhooks create --endpoint https://acme.com/hook --event delivered,bounced,complained`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			evs, err := parseEvents(events)
			if err != nil {
				return err
			}
			if len(evs) == 0 {
				return usageErrorf("give at least one --event (or --event all)")
			}
			client, err := a.client()
			if err != nil {
				return err
			}
			res, err := client.Webhooks.Create(cmd.Context(), &agdiremail.CreateWebhookRequest{Endpoint: endpoint, Events: evs})
			if err != nil {
				return err
			}
			switch {
			case a.opts.json:
				return a.printJSON(res)
			case a.opts.quiet:
				a.printIDs(res.ID)
			default:
				a.printf("Created webhook %s.\nSigning secret: %s\nKeep the secret to verify incoming events (it is also shown by \"agdir-email webhooks get %s\").\n",
					res.ID, res.SigningSecret, res.ID)
			}
			return nil
		},
	}
	c.Flags().StringVar(&endpoint, "endpoint", "", "public https:// URL that receives events (required)")
	c.Flags().StringSliceVar(&events, "event", nil, `event to subscribe to, e.g. delivered or email.bounced (repeatable or comma-separated; "all" for every event) (required)`)
	_ = c.MarkFlagRequired("endpoint")
	_ = c.MarkFlagRequired("event")
	return c
}

func (a *App) newWebhooksListCmd() *cobra.Command {
	var lf listFlags
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List webhook endpoints",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			page, err := fetchList(cmd.Context(), lf, client.Webhooks.List, client.Webhooks.ListAll)
			if err != nil {
				return err
			}
			if a.opts.json {
				return a.printJSON(page)
			}
			if a.opts.quiet {
				for _, w := range page.Data {
					a.printIDs(w.ID)
				}
				return nil
			}
			if len(page.Data) == 0 {
				a.printf("No webhooks yet. Add one with \"agdir-email webhooks create\".\n")
				return nil
			}
			rows := make([][]string, len(page.Data))
			for i, w := range page.Data {
				rows[i] = []string{w.ID, w.Endpoint, string(w.Status), joinEvents(w.Events), formatTime(w.CreatedAt)}
			}
			table(a.Stdout, []string{"ID", "ENDPOINT", "STATUS", "EVENTS", "CREATED"}, rows)
			a.printPageFooter(cmd, lf, page.NextCursor)
			return nil
		},
	}
	lf.register(c)
	return c
}

func (a *App) newWebhooksGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <webhook-id>",
		Short: "Show a webhook endpoint, including its signing secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			w, err := client.Webhooks.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if a.opts.json {
				return a.printJSON(w)
			}
			if a.opts.quiet {
				a.printIDs(w.ID)
				return nil
			}
			fields(a.Stdout, [][2]string{
				{"ID", w.ID},
				{"Endpoint", w.Endpoint},
				{"Status", string(w.Status)},
				{"Events", joinEvents(w.Events)},
				{"Created", formatTime(w.CreatedAt)},
				{"Signing secret", w.SigningSecret},
			})
			return nil
		},
	}
}

func (a *App) newWebhooksUpdateCmd() *cobra.Command {
	var endpoint, status string
	var events []string
	c := &cobra.Command{
		Use:     "update <webhook-id>",
		Short:   "Change a webhook's endpoint, events or status",
		Example: "  agdir-email webhooks update wh_123 --status disabled\n  agdir-email webhooks update wh_123 --event all",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := &agdiremail.UpdateWebhookRequest{Endpoint: endpoint}
			if cmd.Flags().Changed("event") {
				evs, err := parseEvents(events)
				if err != nil {
					return err
				}
				if len(evs) == 0 {
					return usageErrorf("--event needs at least one event")
				}
				req.Events = evs
			}
			switch status {
			case "":
			case "enabled", "disabled":
				req.Status = agdiremail.WebhookStatus(status)
			default:
				return usageErrorf("--status must be enabled or disabled")
			}
			if req.Endpoint == "" && req.Events == nil && req.Status == "" {
				return usageErrorf("nothing to update: give --endpoint, --event or --status")
			}
			client, err := a.client()
			if err != nil {
				return err
			}
			res, err := client.Webhooks.Update(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			switch {
			case a.opts.json:
				return a.printJSON(res)
			case a.opts.quiet:
				a.printIDs(res.ID)
			default:
				a.printf("Updated webhook %s.\n", res.ID)
			}
			return nil
		},
	}
	c.Flags().StringVar(&endpoint, "endpoint", "", "new https:// endpoint URL")
	c.Flags().StringSliceVar(&events, "event", nil, `replace the subscribed events (repeatable or comma-separated; "all")`)
	c.Flags().StringVar(&status, "status", "", "enabled or disabled")
	return c
}

func (a *App) newWebhooksDeleteCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:     "delete <webhook-id>",
		Aliases: []string{"rm"},
		Short:   "Delete a webhook endpoint",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.confirm(yes, fmt.Sprintf("Delete webhook %s?", args[0])); err != nil {
				return err
			}
			client, err := a.client()
			if err != nil {
				return err
			}
			res, err := client.Webhooks.Delete(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			switch {
			case a.opts.json:
				return a.printJSON(res)
			case a.opts.quiet:
				a.printIDs(res.ID)
			default:
				a.printf("Deleted webhook %s.\n", res.ID)
			}
			return nil
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask for confirmation (required when not interactive)")
	return c
}

func (a *App) newWebhooksRotateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate-secret <webhook-id>",
		Short: "Replace a webhook's signing secret",
		Long:  "Generate a new signing secret. Update your endpoint's secret right away: new events are signed with the new one.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			res, err := client.Webhooks.RotateSecret(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			switch {
			case a.opts.json:
				return a.printJSON(res)
			case a.opts.quiet:
				fmt.Fprintln(a.Stdout, res.SigningSecret)
			default:
				a.printf("Rotated the signing secret of webhook %s.\nNew signing secret: %s\n", res.ID, res.SigningSecret)
			}
			return nil
		},
	}
}

func (a *App) newDeliveriesCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "deliveries",
		Aliases: []string{"events"},
		Short:   "Inspect and replay what was sent to a webhook endpoint",
	}

	var lf listFlags
	list := &cobra.Command{
		Use:     "list <webhook-id>",
		Aliases: []string{"ls"},
		Short:   "List the events sent (or queued) to an endpoint",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			d := client.Webhooks.Deliveries
			page, err := fetchList(cmd.Context(), lf,
				func(ctx context.Context, p *agdiremail.ListParams) (*agdiremail.Page[agdiremail.WebhookDelivery], error) {
					return d.List(ctx, args[0], p)
				},
				func(ctx context.Context, p *agdiremail.ListParams) iter.Seq2[agdiremail.WebhookDelivery, error] {
					return d.ListAll(ctx, args[0], p)
				})
			if err != nil {
				return err
			}
			if a.opts.json {
				return a.printJSON(page)
			}
			if a.opts.quiet {
				for _, x := range page.Data {
					a.printIDs(x.ID)
				}
				return nil
			}
			if len(page.Data) == 0 {
				a.printf("No deliveries yet.\n")
				return nil
			}
			rows := make([][]string, len(page.Data))
			for i, x := range page.Data {
				rows[i] = []string{x.ID, string(x.Type), string(x.Status), formatTime(x.CreatedAt)}
			}
			table(a.Stdout, []string{"ID", "EVENT", "STATUS", "CREATED"}, rows)
			a.printPageFooter(cmd, lf, page.NextCursor)
			return nil
		},
	}
	lf.register(list)

	get := &cobra.Command{
		Use:   "get <webhook-id> <delivery-id>",
		Short: "Show a delivery and the exact payload sent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			x, err := client.Webhooks.Deliveries.Get(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			if a.opts.json {
				return a.printJSON(x)
			}
			if a.opts.quiet {
				a.printIDs(x.ID)
				return nil
			}
			fields(a.Stdout, [][2]string{
				{"ID", x.ID},
				{"Event", string(x.Type)},
				{"Status", string(x.Status)},
				{"Created", formatTime(x.CreatedAt)},
				{"Next attempt", formatTimePtr(x.NextAttemptAt)},
			})
			if len(x.Payload) > 0 {
				var buf bytes.Buffer
				if json.Indent(&buf, x.Payload, "", "  ") != nil {
					buf.Reset()
					buf.Write(x.Payload)
				}
				a.printf("\nPayload:\n%s\n", buf.String())
			}
			return nil
		},
	}

	var alf listFlags
	attempts := &cobra.Command{
		Use:   "attempts <webhook-id> <delivery-id>",
		Short: "List the HTTP attempts of a delivery",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			if alf.limit < 0 || alf.limit > 100 {
				return usageErrorf("--limit must be between 1 and 100")
			}
			d := client.Webhooks.Deliveries
			page := &agdiremail.Page[agdiremail.WebhookAttempt]{Data: []agdiremail.WebhookAttempt{}}
			params := &agdiremail.ListParams{Limit: alf.limit, After: alf.after}
			if alf.limit == 0 {
				params.Limit = 100
			}
			for {
				p, err := d.Attempts(cmd.Context(), args[0], args[1], params)
				if err != nil {
					return err
				}
				page.Data = append(page.Data, p.Data...)
				page.HasMore, page.NextCursor = p.HasMore, p.NextCursor
				if alf.limit > 0 || alf.after != "" || p.NextCursor == "" {
					break
				}
				params.After = p.NextCursor
			}
			if a.opts.json {
				return a.printJSON(page)
			}
			if a.opts.quiet {
				for _, x := range page.Data {
					a.printIDs(x.ID)
				}
				return nil
			}
			if len(page.Data) == 0 {
				a.printf("No attempts yet.\n")
				return nil
			}
			rows := make([][]string, len(page.Data))
			for i, x := range page.Data {
				status := "no response"
				if x.HTTPStatus != 0 {
					status = strconv.Itoa(x.HTTPStatus)
				}
				rows[i] = []string{x.ID, status, formatTime(x.SentAt), orDash(truncate(x.Response, 60))}
			}
			table(a.Stdout, []string{"ID", "HTTP", "SENT", "RESPONSE"}, rows)
			a.printPageFooter(cmd, alf, page.NextCursor)
			return nil
		},
	}
	alf.register(attempts)

	replay := &cobra.Command{
		Use:   "replay <webhook-id> <delivery-id>",
		Short: "Send a delivery to the endpoint again",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			res, err := client.Webhooks.Deliveries.Replay(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			switch {
			case a.opts.json:
				return a.printJSON(res)
			case a.opts.quiet:
				a.printIDs(res.ID)
			default:
				a.printf("Replaying delivery %s. Follow it with: agdir-email webhooks deliveries attempts %s %s\n", res.ID, args[0], res.ID)
			}
			return nil
		},
	}
	c.AddCommand(list, get, attempts, replay)
	return c
}

func (a *App) newWebhooksVerifyCmd() *cobra.Command {
	var secret, headersFile string
	var headerFlags []string
	var tolerance time.Duration
	var ignoreTimestamp bool
	c := &cobra.Command{
		Use:   "verify <payload-file>",
		Short: "Check the signature of a captured webhook request",
		Long: `Verify a webhook request you captured (e.g. from your server's logs) the same way your
endpoint should: the raw body in <payload-file> ("-" for stdin) and the webhook-id,
webhook-timestamp and webhook-signature headers.

Headers come from --headers-file (lines of "Name: value", as copied from a request dump,
or a JSON object) and/or --header. The secret comes from --secret or
$AGDIR_EMAIL_WEBHOOK_SECRET. Exits with code 1 if the signature is invalid.`,
		Example: `  agdir-email webhooks verify --secret whsec_… --headers-file headers.txt body.json
  agdir-email webhooks verify --header "webhook-id: msg_1" --header "webhook-timestamp: 1761800000" \
    --header "webhook-signature: v1,K5o…" --ignore-timestamp body.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if secret == "" {
				secret = a.getenv(EnvWebhookSecret)
			}
			if secret == "" {
				return usageErrorf("give the endpoint's signing secret with --secret or $%s", EnvWebhookSecret)
			}
			headers := http.Header{}
			if headersFile != "" {
				if headersFile == "-" && args[0] == "-" {
					return usageErrorf("the payload and the headers can't both come from stdin")
				}
				data, err := a.readInput(headersFile)
				if err != nil {
					return err
				}
				if err := parseHeaders(data, headers); err != nil {
					return usageErrorf("%s: %v", headersFile, err)
				}
			}
			for _, h := range headerFlags {
				if err := parseHeaders([]byte(h), headers); err != nil {
					return usageErrorf("--header %q: %v", h, err)
				}
			}
			payload, err := a.readInput(args[0])
			if err != nil {
				return err
			}

			opts := []agdiremail.VerifyOption{agdiremail.WithTolerance(tolerance), agdiremail.WithVerifyTime(a.now())}
			if ignoreTimestamp {
				if ts, err := strconv.ParseInt(headers.Get("webhook-timestamp"), 10, 64); err == nil {
					opts = append(opts, agdiremail.WithVerifyTime(time.Unix(ts, 0)))
				}
			}
			event, err := agdiremail.VerifyWebhook(payload, headers, secret, opts...)
			var verr *agdiremail.WebhookVerificationError
			if errors.As(err, &verr) {
				expected := expectedSignature(secret, headers, payload)
				if a.opts.json {
					out := map[string]any{"valid": false, "error": verr.Reason}
					if expected != "" {
						out["expected_signature"] = expected
					}
					if err := a.printJSON(out); err != nil {
						return err
					}
				} else if !a.opts.quiet {
					fmt.Fprintf(a.Stdout, "✗ Invalid: %s\n", verr.Reason)
					if expected != "" && strings.Contains(verr.Reason, "signature") {
						fmt.Fprintf(a.Stdout, "  Expected webhook-signature: %s\n", expected)
						fmt.Fprintf(a.Stdout, "  Got:                        %s\n", headers.Get("webhook-signature"))
						fmt.Fprintln(a.Stdout, "  Check that the body is byte-for-byte what was received and that the secret is the endpoint's current one.")
					}
					if strings.Contains(verr.Reason, "tolerance") {
						fmt.Fprintln(a.Stdout, "  Use --ignore-timestamp to check an old capture.")
					}
				}
				return silentFailure(err)
			}
			if err != nil {
				return err
			}
			if a.opts.json {
				return a.printJSON(map[string]any{"valid": true, "event": event})
			}
			if a.opts.quiet {
				return nil
			}
			fmt.Fprintln(a.Stdout, "✓ Signature valid")
			fields(a.Stdout, [][2]string{
				{"  Event", string(event.Type)},
				{"  Email", event.Data.EmailID},
				{"  From", event.Data.From},
				{"  To", strings.Join(event.Data.To, ", ")},
				{"  Subject", event.Data.Subject},
				{"  Created", formatTime(event.CreatedAt)},
			})
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&secret, "secret", "", "the endpoint's signing secret, whsec_… (default: $AGDIR_EMAIL_WEBHOOK_SECRET)")
	f.StringVar(&headersFile, "headers-file", "", `file with the request headers ("Name: value" lines or a JSON object; "-" for stdin)`)
	f.StringArrayVar(&headerFlags, "header", nil, `a request header as "Name: value" (repeatable)`)
	f.DurationVar(&tolerance, "tolerance", agdiremail.DefaultWebhookTolerance, "maximum age of the webhook-timestamp")
	f.BoolVar(&ignoreTimestamp, "ignore-timestamp", false, "skip the timestamp age check (for old captures)")
	return c
}

// parseHeaders reads "Name: value" lines (skipping an HTTP request/status line, blank lines and
// # comments) or a JSON object of header values.
func parseHeaders(data []byte, h http.Header) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("{")) {
		var m map[string]any
		if err := json.Unmarshal(trimmed, &m); err != nil {
			return fmt.Errorf("invalid JSON: %v", err)
		}
		for k, v := range m {
			switch v := v.(type) {
			case string:
				h.Add(k, v)
			case float64:
				h.Add(k, strconv.FormatFloat(v, 'f', -1, 64))
			case []any:
				for _, x := range v {
					h.Add(k, fmt.Sprint(x))
				}
			default:
				return fmt.Errorf("header %q: expected a string", k)
			}
		}
		return nil
	}
	n := 0
	for _, line := range strings.Split(string(trimmed), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.Contains(line, " HTTP/") || strings.HasPrefix(line, "HTTP/") {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return fmt.Errorf(`expected "Name: value", got %q`, line)
		}
		h.Add(strings.TrimSpace(name), strings.TrimSpace(value))
		n++
	}
	if n == 0 {
		return fmt.Errorf("no headers found")
	}
	return nil
}

// expectedSignature computes the v1 signature for the request, for debugging mismatches.
func expectedSignature(secret string, h http.Header, payload []byte) string {
	id, ts := h.Get("webhook-id"), h.Get("webhook-timestamp")
	if id == "" || ts == "" {
		return ""
	}
	s := strings.TrimPrefix(secret, "whsec_")
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		if key, err = base64.RawStdEncoding.DecodeString(s); err != nil {
			return ""
		}
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "."))
	mac.Write(payload)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
