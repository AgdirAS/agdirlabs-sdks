package com.agdirlabs.email;

import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.net.http.HttpTimeoutException;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.ZonedDateTime;
import java.time.format.DateTimeFormatter;
import java.time.format.DateTimeParseException;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.CancellationException;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionException;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ThreadLocalRandom;
import java.util.concurrent.TimeUnit;
import java.util.function.Function;

/** Sends API requests, retrying rate limits, server errors and network failures when safe. */
final class Http {
  private static final Set<Integer> RETRY_STATUSES = Set.of(408, 500, 502, 503, 504);

  private final String apiKey;
  private final String baseUrl;
  private final HttpClient client;
  private final int maxRetries;
  private final Duration timeout;

  Http(String apiKey, String baseUrl, HttpClient client, int maxRetries, Duration timeout) {
    this.apiKey = apiKey;
    this.baseUrl = baseUrl.replaceAll("/+$", "");
    this.client = client;
    this.maxRetries = maxRetries;
    this.timeout = timeout;
  }

  int maxRetries() {
    return maxRetries;
  }

  /** One API call. {@code retryable}: safe to repeat (GETs, and POSTs that carry an idempotency key). */
  static final class Call {
    final String method;
    final String path;
    final Map<String, Object> query = new LinkedHashMap<>();
    final Map<String, String> headers = new LinkedHashMap<>();
    Object body;
    boolean retryable;

    private Call(String method, String path) {
      this.method = method;
      this.path = path;
    }

    static Call get(String path) {
      return new Call("GET", path).retryable(true);
    }

    static Call post(String path) {
      return new Call("POST", path);
    }

    static Call patch(String path) {
      return new Call("PATCH", path);
    }

    static Call delete(String path) {
      return new Call("DELETE", path);
    }

    Call body(Object body) {
      this.body = body;
      return this;
    }

    Call query(String key, Object value) {
      if (value != null) query.put(key, value);
      return this;
    }

    Call header(String key, String value) {
      if (value != null) headers.put(key, value);
      return this;
    }

    Call retryable(boolean retryable) {
      this.retryable = retryable;
      return this;
    }
  }

  /** Percent-encodes one path segment (like JavaScript's {@code encodeURIComponent}). */
  static String seg(String value) {
    if (value == null || value.isEmpty()) throw new IllegalArgumentException("id must not be empty");
    return encode(value);
  }

  private static String encode(String value) {
    return URLEncoder.encode(value, StandardCharsets.UTF_8).replace("+", "%20");
  }

  <T> CompletableFuture<T> send(Call call, Function<Object, T> mapper) {
    StringBuilder url = new StringBuilder(baseUrl).append(call.path);
    char sep = '?';
    for (Map.Entry<String, Object> q : call.query.entrySet()) {
      url.append(sep).append(encode(q.getKey())).append('=').append(encode(String.valueOf(q.getValue())));
      sep = '&';
    }
    HttpRequest.Builder builder =
        HttpRequest.newBuilder(URI.create(url.toString()))
            .timeout(timeout)
            .header("Authorization", "Bearer " + apiKey)
            .header("User-Agent", AgdirEmail.USER_AGENT)
            .header("Accept", "application/json");
    if (call.body != null) {
      builder.header("Content-Type", "application/json");
      builder.method(call.method, HttpRequest.BodyPublishers.ofString(Json.write(call.body), StandardCharsets.UTF_8));
    } else {
      builder.method(call.method, HttpRequest.BodyPublishers.noBody());
    }
    call.headers.forEach(builder::header);
    HttpRequest request = builder.build();

    CompletableFuture<T> result = new CompletableFuture<>();
    attempt(call, request, 0, result, mapper);
    return result;
  }

  private <T> void attempt(
      Call call, HttpRequest request, int attempt, CompletableFuture<T> result, Function<Object, T> mapper) {
    if (result.isDone()) return; // cancelled by the caller while waiting to retry
    CompletableFuture<HttpResponse<String>> inFlight;
    try {
      inFlight = client.sendAsync(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
    } catch (RuntimeException e) {
      result.completeExceptionally(e);
      return;
    }
    // Propagate caller cancellation to the in-flight request (aborts it on JDK 16+).
    result.whenComplete((v, e) -> {
      if (result.isCancelled()) inFlight.cancel(true);
    });

    boolean canRetry = attempt < maxRetries;
    inFlight.whenComplete(
        (response, failure) -> {
          if (result.isDone()) return;
          if (failure != null) {
            Throwable cause = failure instanceof CompletionException && failure.getCause() != null
                ? failure.getCause()
                : failure;
            if (cause instanceof CancellationException) {
              result.cancel(false);
              return;
            }
            // A timed-out POST may have been accepted: only retry it when it is idempotent.
            if (canRetry && call.retryable) {
              retryLater(backoffMillis(attempt), () -> attempt(call, request, attempt + 1, result, mapper));
              return;
            }
            boolean timedOut = cause instanceof HttpTimeoutException;
            result.completeExceptionally(
                new AgdirEmailException(
                    timedOut
                        ? "Request timed out after " + timeout.toMillis() + " ms"
                        : "Could not reach the Agdir Email API",
                    timedOut ? AgdirEmailException.TIMEOUT : AgdirEmailException.NETWORK_ERROR,
                    0,
                    null,
                    cause));
            return;
          }

          int status = response.statusCode();
          if (status >= 200 && status < 300) {
            try {
              String body = response.body();
              Object json = body == null || body.isBlank() ? new LinkedHashMap<String, Object>() : Json.parse(body);
              result.complete(mapper.apply(json));
            } catch (RuntimeException e) {
              result.completeExceptionally(
                  new AgdirEmailException(
                      "Unexpected response from the Agdir Email API", AgdirEmailException.UNKNOWN_ERROR, status, null, e));
            }
            return;
          }

          AgdirEmailException error = toError(response);
          // Rate limits are refused before any work is done, so they are always safe to retry.
          boolean retry =
              canRetry
                  && (AgdirEmailException.RATE_LIMIT_EXCEEDED.equals(error.getCode())
                      || (call.retryable && RETRY_STATUSES.contains(status)));
          if (!retry) {
            result.completeExceptionally(error);
            return;
          }
          long delay = error.getRetryAfter() != null ? error.getRetryAfter().toMillis() : backoffMillis(attempt);
          retryLater(delay, () -> attempt(call, request, attempt + 1, result, mapper));
        });
  }

  private static void retryLater(long delayMillis, Runnable task) {
    CompletableFuture.delayedExecutor(Math.max(0, delayMillis), TimeUnit.MILLISECONDS).execute(task);
  }

  /** Exponential backoff with jitter: ~0.5 s, 1 s, 2 s … capped at 8 s. */
  static long backoffMillis(int attempt) {
    double base = Math.min(8000, 500 * Math.pow(2, attempt));
    return (long) (base / 2 + ThreadLocalRandom.current().nextDouble() * (base / 2));
  }

  static Duration retryAfter(HttpResponse<?> response) {
    String raw = response.headers().firstValue("retry-after").orElse(null);
    if (raw == null || raw.isBlank()) return null;
    try {
      double seconds = Double.parseDouble(raw.trim());
      if (Double.isNaN(seconds) || Double.isInfinite(seconds)) return null;
      return Duration.ofMillis((long) (Math.max(0, seconds) * 1000));
    } catch (NumberFormatException notSeconds) {
      try {
        ZonedDateTime at = ZonedDateTime.parse(raw.trim(), DateTimeFormatter.RFC_1123_DATE_TIME);
        Duration d = Duration.between(java.time.Instant.now(), at.toInstant());
        return d.isNegative() ? Duration.ZERO : d;
      } catch (DateTimeParseException notADate) {
        return null;
      }
    }
  }

  static AgdirEmailException toError(HttpResponse<String> response) {
    int status = response.statusCode();
    Map<String, Object> body;
    try {
      body = Json.object(Json.parse(response.body() == null ? "" : response.body()));
    } catch (RuntimeException notJson) {
      // Not JSON (e.g. a proxy error page): fall back to the status line.
      body = Map.of();
    }
    String message = Json.string(body, "message");
    String code = Json.string(body, "name");
    return new AgdirEmailException(
        message != null ? message : "Request failed with HTTP " + status,
        code != null
            ? code
            : status >= 500 ? AgdirEmailException.INTERNAL_SERVER_ERROR : AgdirEmailException.UNKNOWN_ERROR,
        status,
        retryAfter(response),
        null);
  }

  /** Waits for an async result, rethrowing its failure on the calling thread. */
  static <T> T await(CompletableFuture<T> future) {
    try {
      return future.get();
    } catch (InterruptedException e) {
      future.cancel(true);
      Thread.currentThread().interrupt();
      CancellationException cancelled = new CancellationException("Interrupted while waiting for the Agdir Email API");
      cancelled.initCause(e);
      throw cancelled;
    } catch (ExecutionException e) {
      Throwable cause = e.getCause();
      if (cause instanceof AgdirEmailException) {
        AgdirEmailException api = (AgdirEmailException) cause;
        // Re-create so the stack trace shows the caller's thread rather than the HTTP client's.
        throw new AgdirEmailException(
            api.getMessage(), api.getCode(), api.getStatus(), api.getRetryAfter(), api.getCause());
      }
      if (cause instanceof RuntimeException) throw (RuntimeException) cause;
      if (cause instanceof Error) throw (Error) cause;
      throw new AgdirEmailException(
          "Could not reach the Agdir Email API", AgdirEmailException.NETWORK_ERROR, 0, null, cause);
    }
  }
}
