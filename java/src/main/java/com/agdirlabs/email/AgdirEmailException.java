package com.agdirlabs.email;

import java.time.Duration;

/**
 * Thrown by every client method when a request fails.
 *
 * <p>{@link #getCode()} is the API's machine-readable error name (e.g. {@code validation_error},
 * {@code rate_limit_exceeded}), or {@code network_error} / {@code timeout} when no response was received.
 */
public class AgdirEmailException extends RuntimeException {
  private static final long serialVersionUID = 1L;

  public static final String MISSING_API_KEY = "missing_api_key";
  public static final String INVALID_API_KEY = "invalid_api_key";
  public static final String RESTRICTED_API_KEY = "restricted_api_key";
  public static final String VALIDATION_ERROR = "validation_error";
  public static final String NOT_FOUND = "not_found";
  public static final String INVALID_IDEMPOTENT_REQUEST = "invalid_idempotent_request";
  public static final String CONCURRENT_IDEMPOTENT_REQUESTS = "concurrent_idempotent_requests";
  public static final String DAILY_QUOTA_EXCEEDED = "daily_quota_exceeded";
  public static final String MONTHLY_QUOTA_EXCEEDED = "monthly_quota_exceeded";
  public static final String RATE_LIMIT_EXCEEDED = "rate_limit_exceeded";
  public static final String INTERNAL_SERVER_ERROR = "internal_server_error";
  public static final String UNKNOWN_ERROR = "unknown_error";
  /** Client-side: the API could not be reached. */
  public static final String NETWORK_ERROR = "network_error";
  /** Client-side: no response within the configured timeout. */
  public static final String TIMEOUT = "timeout";

  private final String code;
  private final int status;
  private final transient Duration retryAfter;

  public AgdirEmailException(String message, String code, int status, Duration retryAfter, Throwable cause) {
    super(message, cause);
    this.code = code;
    this.status = status;
    this.retryAfter = retryAfter;
  }

  public AgdirEmailException(String message, String code, int status) {
    this(message, code, status, null, null);
  }

  /** Machine-readable code, e.g. {@code validation_error}; see the constants on this class. */
  public String getCode() {
    return code;
  }

  /** HTTP status, or 0 when no response was received. */
  public int getStatus() {
    return status;
  }

  /** How long the API asked us to wait (from {@code retry-after}), or {@code null}. */
  public Duration getRetryAfter() {
    return retryAfter;
  }

  @Override
  public String toString() {
    return "AgdirEmailException{code=" + code + ", status=" + status + ", message=" + getMessage() + "}";
  }
}
