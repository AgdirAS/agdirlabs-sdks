package com.agdirlabs.email;

import java.time.Instant;
import java.util.List;
import java.util.Map;

/** An event received on your webhook endpoint, as returned by {@link Webhooks#verify}. */
public final class WebhookEvent {
  private final WebhookEventType type;
  private final String rawType;
  private final Instant createdAt;
  private final Data data;

  WebhookEvent(String rawType, Instant createdAt, Data data) {
    this.type = WebhookEventType.fromValue(rawType);
    this.rawType = rawType;
    this.createdAt = createdAt;
    this.data = data;
  }

  public WebhookEventType getType() {
    return type;
  }

  /** The type exactly as sent, e.g. {@code email.bounced} (useful when {@link #getType()} is UNKNOWN). */
  public String getRawType() {
    return rawType;
  }

  public Instant getCreatedAt() {
    return createdAt;
  }

  public Data getData() {
    return data;
  }

  @Override
  public String toString() {
    return "WebhookEvent{type=" + rawType + ", createdAt=" + createdAt + ", emailId=" + data.getEmailId() + "}";
  }

  /** The email the event is about. */
  public static final class Data {
    private final String emailId;
    private final String messageId;
    private final String from;
    private final List<String> to;
    private final String subject;
    private final Map<String, String> tags;
    private final Instant createdAt;
    private final Bounce bounce;
    private final Click click;
    private final Failure failed;

    Data(String emailId, String messageId, String from, List<String> to, String subject, Map<String, String> tags,
        Instant createdAt, Bounce bounce, Click click, Failure failed) {
      this.emailId = emailId;
      this.messageId = messageId;
      this.from = from;
      this.to = List.copyOf(to);
      this.subject = subject;
      this.tags = Map.copyOf(tags);
      this.createdAt = createdAt;
      this.bounce = bounce;
      this.click = click;
      this.failed = failed;
    }

    /** Use it to deduplicate: failed deliveries are retried. */
    public String getEmailId() {
      return emailId;
    }

    /** The outgoing message id, or {@code null} before the email was sent. */
    public String getMessageId() {
      return messageId;
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

    /** The email's tags as name → value. */
    public Map<String, String> getTags() {
      return tags;
    }

    /** When the email was created. */
    public Instant getCreatedAt() {
      return createdAt;
    }

    /** Set for {@code email.bounced}, else {@code null}. */
    public Bounce getBounce() {
      return bounce;
    }

    /** Set for {@code email.clicked}, else {@code null}. */
    public Click getClick() {
      return click;
    }

    /** Set for {@code email.failed}, else {@code null}. */
    public Failure getFailed() {
      return failed;
    }
  }

  /** Bounce details. */
  public static final class Bounce {
    private final String type;
    private final String subType;
    private final String message;

    Bounce(String type, String subType, String message) {
      this.type = type;
      this.subType = subType;
      this.message = message;
    }

    /** E.g. {@code Permanent} or {@code Transient}. */
    public String getType() {
      return type;
    }

    /** E.g. {@code General}, {@code MailboxFull}. */
    public String getSubType() {
      return subType;
    }

    public String getMessage() {
      return message;
    }
  }

  /** Click details. */
  public static final class Click {
    private final String link;
    private final String ipAddress;
    private final String userAgent;
    private final Instant timestamp;

    Click(String link, String ipAddress, String userAgent, Instant timestamp) {
      this.link = link;
      this.ipAddress = ipAddress;
      this.userAgent = userAgent;
      this.timestamp = timestamp;
    }

    public String getLink() {
      return link;
    }

    public String getIpAddress() {
      return ipAddress;
    }

    public String getUserAgent() {
      return userAgent;
    }

    public Instant getTimestamp() {
      return timestamp;
    }
  }

  /** Why an email could not be sent. */
  public static final class Failure {
    private final String reason;

    Failure(String reason) {
      this.reason = reason;
    }

    public String getReason() {
      return reason;
    }
  }
}
