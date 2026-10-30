package com.agdirlabs.email;

import java.time.Instant;
import java.time.OffsetDateTime;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/**
 * One email to send. Immutable; create it with {@link #builder()}.
 *
 * <pre>{@code
 * SendEmailRequest email = SendEmailRequest.builder()
 *     .from("Acme <hello@acme.com>")
 *     .to("user@example.com")
 *     .subject("Welcome")
 *     .html("<p>Thanks for signing up!</p>")
 *     .build();
 * }</pre>
 *
 * <p>Addresses are {@code "Name <addr@example.com>"} or bare addresses.
 */
public final class SendEmailRequest {
  private final String from;
  private final List<String> to;
  private final String subject;
  private final String html;
  private final String text;
  private final List<String> cc;
  private final List<String> bcc;
  private final List<String> replyTo;
  private final Map<String, String> headers;
  private final List<Tag> tags;
  private final String scheduledAt;

  private SendEmailRequest(Builder b) {
    this.from = Objects.requireNonNull(b.from, "from is required");
    if (b.to.isEmpty()) throw new IllegalArgumentException("to is required");
    this.to = List.copyOf(b.to);
    this.subject = Objects.requireNonNull(b.subject, "subject is required");
    if (b.html == null && b.text == null) throw new IllegalArgumentException("html or text is required");
    this.html = b.html;
    this.text = b.text;
    this.cc = List.copyOf(b.cc);
    this.bcc = List.copyOf(b.bcc);
    this.replyTo = List.copyOf(b.replyTo);
    this.headers = Collections.unmodifiableMap(new LinkedHashMap<>(b.headers));
    this.tags = List.copyOf(b.tags);
    this.scheduledAt = b.scheduledAt;
  }

  public static Builder builder() {
    return new Builder();
  }

  /** A builder pre-filled with this request's values, for sending a variation. */
  public Builder toBuilder() {
    Builder b = new Builder();
    b.from = from;
    b.to.addAll(to);
    b.subject = subject;
    b.html = html;
    b.text = text;
    b.cc.addAll(cc);
    b.bcc.addAll(bcc);
    b.replyTo.addAll(replyTo);
    b.headers.putAll(headers);
    b.tags.addAll(tags);
    b.scheduledAt = scheduledAt;
    return b;
  }

  public String getFrom() {
    return from;
  }

  public List<String> getTo() {
    return to;
  }

  public String getSubject() {
    return subject;
  }

  /** May be {@code null}. */
  public String getHtml() {
    return html;
  }

  /** May be {@code null}. */
  public String getText() {
    return text;
  }

  public List<String> getCc() {
    return cc;
  }

  public List<String> getBcc() {
    return bcc;
  }

  public List<String> getReplyTo() {
    return replyTo;
  }

  public Map<String, String> getHeaders() {
    return headers;
  }

  public List<Tag> getTags() {
    return tags;
  }

  /** ISO 8601 timestamp with offset, or {@code null} to send now. */
  public String getScheduledAt() {
    return scheduledAt;
  }

  /** The API's JSON body. */
  Map<String, Object> toWire() {
    Map<String, Object> m = new LinkedHashMap<>();
    m.put("from", from);
    m.put("to", to);
    m.put("subject", subject);
    if (html != null) m.put("html", html);
    if (text != null) m.put("text", text);
    if (!cc.isEmpty()) m.put("cc", cc);
    if (!bcc.isEmpty()) m.put("bcc", bcc);
    if (!replyTo.isEmpty()) m.put("reply_to", replyTo);
    if (!headers.isEmpty()) m.put("headers", headers);
    if (!tags.isEmpty()) {
      List<Map<String, Object>> t = new ArrayList<>();
      for (Tag tag : tags) {
        Map<String, Object> e = new LinkedHashMap<>();
        e.put("name", tag.getName());
        e.put("value", tag.getValue());
        t.add(e);
      }
      m.put("tags", t);
    }
    if (scheduledAt != null) m.put("scheduled_at", scheduledAt);
    return m;
  }

  @Override
  public String toString() {
    return "SendEmailRequest{from=" + from + ", to=" + to + ", subject=" + subject + "}";
  }

  /** Mutable builder; {@link #build()} validates that from, to, subject and html or text are set. */
  public static final class Builder {
    private String from;
    private final List<String> to = new ArrayList<>();
    private String subject;
    private String html;
    private String text;
    private final List<String> cc = new ArrayList<>();
    private final List<String> bcc = new ArrayList<>();
    private final List<String> replyTo = new ArrayList<>();
    private final Map<String, String> headers = new LinkedHashMap<>();
    private final List<Tag> tags = new ArrayList<>();
    private String scheduledAt;

    private Builder() {}

    public Builder from(String from) {
      this.from = from;
      return this;
    }

    /** Replaces the recipients. */
    public Builder to(String... to) {
      return to(Arrays.asList(to));
    }

    /** Replaces the recipients. */
    public Builder to(Collection<String> to) {
      return replace(this.to, to);
    }

    public Builder subject(String subject) {
      this.subject = subject;
      return this;
    }

    public Builder html(String html) {
      this.html = html;
      return this;
    }

    public Builder text(String text) {
      this.text = text;
      return this;
    }

    public Builder cc(String... cc) {
      return cc(Arrays.asList(cc));
    }

    public Builder cc(Collection<String> cc) {
      return replace(this.cc, cc);
    }

    public Builder bcc(String... bcc) {
      return bcc(Arrays.asList(bcc));
    }

    public Builder bcc(Collection<String> bcc) {
      return replace(this.bcc, bcc);
    }

    public Builder replyTo(String... replyTo) {
      return replyTo(Arrays.asList(replyTo));
    }

    public Builder replyTo(Collection<String> replyTo) {
      return replace(this.replyTo, replyTo);
    }

    /** Adds one custom header, e.g. {@code X-Entity-Ref-ID}. */
    public Builder header(String name, String value) {
      headers.put(Objects.requireNonNull(name, "name"), Objects.requireNonNull(value, "value"));
      return this;
    }

    /** Replaces the custom headers. */
    public Builder headers(Map<String, String> headers) {
      this.headers.clear();
      headers.forEach(this::header);
      return this;
    }

    /** Adds one tag. */
    public Builder tag(String name, String value) {
      tags.add(Tag.of(name, value));
      return this;
    }

    /** Replaces the tags. */
    public Builder tags(Collection<Tag> tags) {
      this.tags.clear();
      for (Tag t : tags) this.tags.add(Objects.requireNonNull(t, "tag"));
      return this;
    }

    /** Deliver later instead of now. */
    public Builder scheduledAt(Instant at) {
      this.scheduledAt = at == null ? null : DateTimeFormatter.ISO_INSTANT.format(at);
      return this;
    }

    /** Deliver later instead of now. */
    public Builder scheduledAt(OffsetDateTime at) {
      this.scheduledAt = at == null ? null : DateTimeFormatter.ISO_OFFSET_DATE_TIME.format(at);
      return this;
    }

    /** Deliver later: an ISO 8601 timestamp with offset, e.g. {@code 2026-11-01T09:00:00+01:00}. */
    public Builder scheduledAt(String at) {
      this.scheduledAt = at;
      return this;
    }

    public SendEmailRequest build() {
      return new SendEmailRequest(this);
    }

    private Builder replace(List<String> target, Collection<String> values) {
      target.clear();
      if (values != null) {
        for (String v : values) target.add(Objects.requireNonNull(v, "address"));
      }
      return this;
    }
  }
}
