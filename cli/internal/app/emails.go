package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
	"github.com/spf13/cobra"
)

type sendFlags struct {
	from, subject            string
	to, cc, bcc, replyTo     []string
	html, htmlFile           string
	text, textFile           string
	tags, headers            []string
	schedule, idempotencyKey string
}

func (a *App) newSendCmd() *cobra.Command {
	var f sendFlags
	c := &cobra.Command{
		Use:   "send",
		Short: "Send an email",
		Long: `Send an email (or schedule it with --schedule).

The body comes from --html/--text, from files (--html-file/--text-file, "-" for stdin),
or, when no body flag is given, from stdin as plain text. Give both an HTML and a text
part for the best deliverability. The sender's domain must be verified.`,
		Example: `  agdir-email send --from "Acme <hi@acme.com>" --to ana@example.com \
    --subject "Welcome" --html "<p>Thanks for signing up!</p>"

  # Body from a file, plain-text part from stdin, tags and a custom header
  ./render-report | agdir-email send --from reports@acme.com --to team@acme.com \
    --subject "Weekly report" --html-file report.html --text-file - \
    --tag kind=report --header "X-Entity-Ref-ID: week-44"

  # Schedule for later (RFC 3339 or a relative time)
  agdir-email send --from hi@acme.com --to ana@example.com --subject Reminder \
    --text "See you tomorrow" --schedule "in 2h"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			email, err := a.buildEmail(&f)
			if err != nil {
				return err
			}
			client, err := a.client()
			if err != nil {
				return err
			}
			var opts []agdiremail.RequestOption
			if f.idempotencyKey != "" {
				opts = append(opts, agdiremail.WithIdempotencyKey(f.idempotencyKey))
			}
			res, err := client.Emails.Send(cmd.Context(), email, opts...)
			if err != nil {
				return err
			}
			switch {
			case a.opts.json:
				out := map[string]any{"id": res.ID}
				if !email.ScheduledAt.IsZero() {
					out["scheduled_at"] = email.ScheduledAt.UTC().Format(time.RFC3339)
				}
				return a.printJSON(out)
			case a.opts.quiet:
				a.printIDs(res.ID)
			case !email.ScheduledAt.IsZero():
				a.printf("Scheduled email %s for %s.\n", res.ID, formatTime(email.ScheduledAt))
			default:
				a.printf("Sent email %s.\n", res.ID)
			}
			return nil
		},
	}
	fl := c.Flags()
	fl.StringVar(&f.from, "from", "", `sender, "Name <addr@your-domain>" or a bare address (required)`)
	fl.StringArrayVar(&f.to, "to", nil, "recipient (repeatable, up to 50) (required)")
	fl.StringVar(&f.subject, "subject", "", "subject line (required)")
	fl.StringVar(&f.html, "html", "", "HTML body")
	fl.StringVar(&f.htmlFile, "html-file", "", `read the HTML body from a file ("-" for stdin)`)
	fl.StringVar(&f.text, "text", "", "plain-text body")
	fl.StringVar(&f.textFile, "text-file", "", `read the plain-text body from a file ("-" for stdin)`)
	fl.StringArrayVar(&f.cc, "cc", nil, "Cc recipient (repeatable)")
	fl.StringArrayVar(&f.bcc, "bcc", nil, "Bcc recipient (repeatable)")
	fl.StringArrayVar(&f.replyTo, "reply-to", nil, "Reply-To address (repeatable)")
	fl.StringArrayVar(&f.tags, "tag", nil, "tag as name=value, echoed in webhook events (repeatable)")
	fl.StringArrayVar(&f.headers, "header", nil, `custom header as "Name: value" (repeatable)`)
	fl.StringVar(&f.schedule, "schedule", "", `send later: RFC 3339 time ("2026-11-02T09:00:00+01:00") or relative ("in 30m", "in 2 days")`)
	fl.StringVar(&f.idempotencyKey, "idempotency-key", "", "make the send safe to repeat: the same key returns the original email instead of sending again")
	_ = c.MarkFlagRequired("from")
	_ = c.MarkFlagRequired("to")
	_ = c.MarkFlagRequired("subject")
	c.MarkFlagsMutuallyExclusive("html", "html-file")
	c.MarkFlagsMutuallyExclusive("text", "text-file")
	return c
}

func (a *App) buildEmail(f *sendFlags) (*agdiremail.SendEmailRequest, error) {
	email := &agdiremail.SendEmailRequest{
		From: f.from, To: f.to, Subject: f.subject, Cc: f.cc, Bcc: f.bcc, ReplyTo: f.replyTo,
		HTML: f.html, Text: f.text,
	}
	if f.htmlFile == "-" && f.textFile == "-" {
		return nil, usageErrorf("only one of --html-file and --text-file can read stdin")
	}
	if f.htmlFile != "" {
		b, err := a.readInput(f.htmlFile)
		if err != nil {
			return nil, err
		}
		email.HTML = string(b)
	}
	if f.textFile != "" {
		b, err := a.readInput(f.textFile)
		if err != nil {
			return nil, err
		}
		email.Text = string(b)
	}
	if email.HTML == "" && email.Text == "" && f.html == "" && f.text == "" && f.htmlFile == "" && f.textFile == "" && !a.stdinIsTerminal() {
		b, err := a.readInput("-")
		if err != nil {
			return nil, err
		}
		email.Text = string(b)
	}
	if strings.TrimSpace(email.HTML) == "" && strings.TrimSpace(email.Text) == "" {
		return nil, usageErrorf("no body: use --html, --html-file, --text or --text-file, or pipe the text on stdin")
	}

	for _, t := range f.tags {
		name, value, ok := strings.Cut(t, "=")
		if !ok || name == "" {
			return nil, usageErrorf("invalid --tag %q: use name=value", t)
		}
		email.Tags = append(email.Tags, agdiremail.Tag{Name: name, Value: value})
	}
	for _, h := range f.headers {
		name, value, ok := strings.Cut(h, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" || strings.ContainsAny(name, " \t") {
			return nil, usageErrorf(`invalid --header %q: use "Name: value"`, h)
		}
		if email.Headers == nil {
			email.Headers = map[string]string{}
		}
		email.Headers[name] = strings.TrimSpace(value)
	}
	if f.schedule != "" {
		t, err := parseSchedule(f.schedule, a.now())
		if err != nil {
			return nil, usageError(err)
		}
		email.ScheduledAt = t
	}
	return email, nil
}

var (
	durationPart = regexp.MustCompile(`^(\d+)\s*([a-z]+)\s*,?\s*(and\s+)?`)
	unitDuration = map[string]time.Duration{
		"s": time.Second, "sec": time.Second, "secs": time.Second, "second": time.Second, "seconds": time.Second,
		"m": time.Minute, "min": time.Minute, "mins": time.Minute, "minute": time.Minute, "minutes": time.Minute,
		"h": time.Hour, "hr": time.Hour, "hrs": time.Hour, "hour": time.Hour, "hours": time.Hour,
		"d": 24 * time.Hour, "day": 24 * time.Hour, "days": 24 * time.Hour,
		"w": 7 * 24 * time.Hour, "week": 7 * 24 * time.Hour, "weeks": 7 * 24 * time.Hour,
	}
)

// parseSchedule accepts an RFC 3339 time or "in <duration>" ("in 1h", "in 90m", "in 2 days",
// "in 1 day 3 hours").
func parseSchedule(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	lower := strings.ToLower(s)
	if rest, ok := strings.CutPrefix(lower, "in "); ok {
		if d, err := parseRelative(strings.TrimSpace(rest)); err == nil && d > 0 {
			return now.Add(d).Truncate(time.Second), nil
		}
	}
	return time.Time{}, fmt.Errorf(`invalid --schedule %q: use an RFC 3339 time such as "2026-11-02T09:00:00+01:00" or a relative time such as "in 1h" or "in 2 days"`, s)
}

func parseRelative(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	var total time.Duration
	for s != "" {
		m := durationPart.FindStringSubmatch(s)
		if m == nil {
			return 0, fmt.Errorf("invalid duration")
		}
		n, err := strconv.Atoi(m[1])
		unit, ok := unitDuration[m[2]]
		if err != nil || !ok {
			return 0, fmt.Errorf("invalid duration")
		}
		total += time.Duration(n) * unit
		s = s[len(m[0]):]
	}
	return total, nil
}

func (a *App) newEmailsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "emails",
		Short: "Inspect and cancel sent emails",
	}
	c.AddCommand(&cobra.Command{
		Use:   "get <email-id>",
		Short: "Show an email and its delivery status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			e, err := client.Emails.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if a.opts.json {
				return a.printJSON(e)
			}
			if a.opts.quiet {
				a.printIDs(e.ID)
				return nil
			}
			var body []string
			if e.HTML != "" {
				body = append(body, fmt.Sprintf("HTML (%d bytes)", len(e.HTML)))
			}
			if e.Text != "" {
				body = append(body, fmt.Sprintf("text (%d bytes)", len(e.Text)))
			}
			fields(a.Stdout, [][2]string{
				{"ID", e.ID},
				{"Status", string(e.Status)},
				{"From", e.From},
				{"To", strings.Join(e.To, ", ")},
				{"Cc", strings.Join(e.Cc, ", ")},
				{"Bcc", strings.Join(e.Bcc, ", ")},
				{"Reply-To", strings.Join(e.ReplyTo, ", ")},
				{"Subject", e.Subject},
				{"Body", strings.Join(body, ", ")},
				{"Created", formatTime(e.CreatedAt)},
				{"Scheduled", formatTimePtr(e.ScheduledAt)},
			})
			return nil
		},
	}, &cobra.Command{
		Use:   "cancel <email-id>",
		Short: "Cancel a scheduled email",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			res, err := client.Emails.Cancel(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			switch {
			case a.opts.json:
				return a.printJSON(map[string]any{"id": res.ID, "canceled": true})
			case a.opts.quiet:
				a.printIDs(res.ID)
			default:
				a.printf("Canceled email %s.\n", res.ID)
			}
			return nil
		},
	})
	return c
}

// batchEmail is one email of a batch file, in the API's JSON format.
type batchEmail struct {
	From        string            `json:"from"`
	To          stringList        `json:"to"`
	Subject     string            `json:"subject"`
	HTML        string            `json:"html"`
	Text        string            `json:"text"`
	Cc          stringList        `json:"cc"`
	Bcc         stringList        `json:"bcc"`
	ReplyTo     stringList        `json:"reply_to"`
	Headers     map[string]string `json:"headers"`
	Tags        []agdiremail.Tag  `json:"tags"`
	ScheduledAt string            `json:"scheduled_at"`
}

// stringList accepts a JSON string or an array of strings.
type stringList []string

func (l *stringList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*l = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("expected a string or an array of strings")
	}
	*l = many
	return nil
}

func (a *App) newBatchCmd() *cobra.Command {
	var permissive bool
	var idempotencyKey string
	send := &cobra.Command{
		Use:   "send <file.json>",
		Short: "Send up to 100 emails in one request",
		Long: `Send up to 100 emails from a JSON file ("-" for stdin): an array of email objects in the
API's format (from, to, subject, html, text, cc, bcc, reply_to, headers, tags, scheduled_at).

By default one invalid email rejects the whole batch. With --permissive the valid emails are
sent and the rejected ones are reported; the command then exits with code 1 if any were rejected.`,
		Example: `  agdir-email batch send emails.json
  jq '[.users[] | {from: "hi@acme.com", to: .email, subject: "Hi", text: "Hello!"}]' users.json \
    | agdir-email batch send - --permissive --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return err
			}
			var items []batchEmail
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&items); err != nil {
				return usageErrorf("%s: expected a JSON array of emails: %v", args[0], err)
			}
			if len(items) == 0 || len(items) > agdiremail.MaxBatchSize {
				return usageErrorf("%s: a batch holds 1 to %d emails, got %d", args[0], agdiremail.MaxBatchSize, len(items))
			}
			emails := make([]agdiremail.SendEmailRequest, len(items))
			for i, it := range items {
				emails[i] = agdiremail.SendEmailRequest{
					From: it.From, To: it.To, Subject: it.Subject, HTML: it.HTML, Text: it.Text,
					Cc: it.Cc, Bcc: it.Bcc, ReplyTo: it.ReplyTo, Headers: it.Headers, Tags: it.Tags,
				}
				if it.ScheduledAt != "" {
					t, err := time.Parse(time.RFC3339, it.ScheduledAt)
					if err != nil {
						return usageErrorf("%s: email %d: scheduled_at must be an RFC 3339 time", args[0], i)
					}
					emails[i].ScheduledAt = t
				}
			}

			client, err := a.client()
			if err != nil {
				return err
			}
			opts := []agdiremail.RequestOption{}
			if permissive {
				opts = append(opts, agdiremail.WithBatchValidation(agdiremail.BatchPermissive))
			}
			if idempotencyKey != "" {
				opts = append(opts, agdiremail.WithIdempotencyKey(idempotencyKey))
			}
			res, err := client.Emails.SendBatch(cmd.Context(), emails, opts...)
			if err != nil {
				return err
			}

			// Accepted emails are listed in request order, minus the rejected ones.
			rejected := map[int]string{}
			for _, e := range res.Errors {
				rejected[e.Index] = e.Message
			}
			type result struct {
				Index int    `json:"index"`
				ID    string `json:"id,omitempty"`
				Error string `json:"error,omitempty"`
			}
			results := make([]result, 0, len(emails))
			next := 0
			for i := range emails {
				if msg, ok := rejected[i]; ok {
					results = append(results, result{Index: i, Error: msg})
				} else if next < len(res.Data) {
					results = append(results, result{Index: i, ID: res.Data[next].ID})
					next++
				}
			}

			switch {
			case a.opts.json:
				if err := a.printJSON(map[string]any{
					"data": nonNil(res.Data), "errors": nonNil(res.Errors), "results": results,
				}); err != nil {
					return err
				}
			case a.opts.quiet:
				for _, r := range res.Data {
					a.printIDs(r.ID)
				}
			default:
				rows := make([][]string, len(results))
				for i, r := range results {
					if r.Error != "" {
						rows[i] = []string{strconv.Itoa(r.Index), "✗", r.Error}
					} else {
						rows[i] = []string{strconv.Itoa(r.Index), "✓", r.ID}
					}
				}
				table(a.Stdout, []string{"#", "", "EMAIL ID / ERROR"}, rows)
				a.printf("\nAccepted %d of %d emails.\n", len(res.Data), len(emails))
			}
			if len(res.Errors) > 0 {
				if !a.opts.json {
					fmt.Fprintf(a.Stderr, "%d of %d emails were rejected.\n", len(res.Errors), len(emails))
				}
				return silentFailure(fmt.Errorf("%d emails rejected", len(res.Errors)))
			}
			return nil
		},
	}
	send.Flags().BoolVar(&permissive, "permissive", false, "send the valid emails and report the invalid ones instead of rejecting the batch")
	send.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "make the batch safe to repeat")

	c := &cobra.Command{
		Use:   "batch",
		Short: "Send emails in bulk",
	}
	c.AddCommand(send)
	return c
}
