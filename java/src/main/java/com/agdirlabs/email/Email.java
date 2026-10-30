package com.agdirlabs.email;

import java.time.Instant;
import java.util.List;

/** An email and its current delivery status, as returned by {@link Emails#get}. */
public final class Email {
  private final String id;
  private final String from;
  private final List<String> to;
  private final List<String> cc;
  private final List<String> bcc;
  private final List<String> replyTo;
  private final String subject;
  private final String html;
  private final String text;
  private final Instant createdAt;
  private final Instant scheduledAt;
  private final EmailStatus status;

  Email(
      String id,
      String from,
      List<String> to,
      List<String> cc,
      List<String> bcc,
      List<String> replyTo,
      String subject,
      String html,
      String text,
      Instant createdAt,
      Instant scheduledAt,
      EmailStatus status) {
    this.id = id;
    this.from = from;
    this.to = List.copyOf(to);
    this.cc = List.copyOf(cc);
    this.bcc = List.copyOf(bcc);
    this.replyTo = List.copyOf(replyTo);
    this.subject = subject;
    this.html = html;
    this.text = text;
    this.createdAt = createdAt;
    this.scheduledAt = scheduledAt;
    this.status = status;
  }

  public String getId() {
    return id;
  }

  public String getFrom() {
    return from;
  }

  public List<String> getTo() {
    return to;
  }

  /** Empty when none. */
  public List<String> getCc() {
    return cc;
  }

  /** Empty when none. */
  public List<String> getBcc() {
    return bcc;
  }

  /** Empty when none. */
  public List<String> getReplyTo() {
    return replyTo;
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

  public Instant getCreatedAt() {
    return createdAt;
  }

  /** {@code null} unless the email was scheduled. */
  public Instant getScheduledAt() {
    return scheduledAt;
  }

  /** Most advanced delivery state reached so far. */
  public EmailStatus getStatus() {
    return status;
  }

  @Override
  public String toString() {
    return "Email{id=" + id + ", from=" + from + ", to=" + to + ", subject=" + subject + ", status=" + status
        + ", createdAt=" + createdAt + ", scheduledAt=" + scheduledAt + "}";
  }
}
