package com.agdirlabs.email;

import com.sun.net.httpserver.Headers;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.OutputStream;
import java.net.InetAddress;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.Executors;

/** A local HTTP server with scripted replies that records every request. */
final class StubServer implements AutoCloseable {
  static final class Reply {
    final int status;
    final String body;
    final Map<String, String> headers;
    final boolean drop;
    final long delayMillis;

    private Reply(int status, String body, Map<String, String> headers, boolean drop, long delayMillis) {
      this.status = status;
      this.body = body;
      this.headers = headers;
      this.drop = drop;
      this.delayMillis = delayMillis;
    }

    static Reply json(String body) {
      return new Reply(200, body, Map.of(), false, 0);
    }

    static Reply status(int status, String body) {
      return new Reply(status, body, Map.of(), false, 0);
    }

    static Reply status(int status, String body, Map<String, String> headers) {
      return new Reply(status, body, headers, false, 0);
    }

    /** Closes the connection without a response (a network failure). */
    static Reply drop() {
      return new Reply(0, null, Map.of(), true, 0);
    }

    static Reply slow(long delayMillis, String body) {
      return new Reply(200, body, Map.of(), false, delayMillis);
    }
  }

  static final class Recorded {
    final String method;
    final String path;
    final String query;
    final Headers headers;
    final String body;

    Recorded(String method, String path, String query, Headers headers, String body) {
      this.method = method;
      this.path = path;
      this.query = query;
      this.headers = headers;
      this.body = body;
    }

    String header(String name) {
      return headers.getFirst(name);
    }

    Object json() {
      return Json.parse(body);
    }
  }

  private final HttpServer server;
  private final ConcurrentLinkedQueue<Reply> replies = new ConcurrentLinkedQueue<>();
  final List<Recorded> requests = new CopyOnWriteArrayList<>();

  StubServer(Reply... scripted) throws IOException {
    replies.addAll(List.of(scripted));
    server = HttpServer.create(new InetSocketAddress(InetAddress.getLoopbackAddress(), 0), 0);
    server.setExecutor(Executors.newCachedThreadPool());
    server.createContext("/", this::handle);
    server.start();
  }

  String url() {
    return "http://127.0.0.1:" + server.getAddress().getPort();
  }

  AgdirEmail client() {
    return client(2);
  }

  AgdirEmail client(int maxRetries) {
    return AgdirEmail.builder().apiKey("re_test").baseUrl(url()).maxRetries(maxRetries).env(n -> null).build();
  }

  Recorded request(int i) {
    return requests.get(i);
  }

  private void handle(HttpExchange ex) throws IOException {
    String body = new String(ex.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
    requests.add(new Recorded(
        ex.getRequestMethod(), ex.getRequestURI().getRawPath(), ex.getRequestURI().getRawQuery(),
        ex.getRequestHeaders(), body));
    Reply reply = replies.poll();
    if (reply == null) reply = Reply.status(599, "{\"name\":\"stub\",\"message\":\"no reply scripted\"}");
    if (reply.drop) {
      ex.close();
      return;
    }
    if (reply.delayMillis > 0) {
      try {
        Thread.sleep(reply.delayMillis);
      } catch (InterruptedException e) {
        Thread.currentThread().interrupt();
      }
    }
    byte[] bytes = reply.body == null ? new byte[0] : reply.body.getBytes(StandardCharsets.UTF_8);
    ex.getResponseHeaders().set("Content-Type", "application/json");
    reply.headers.forEach((k, v) -> ex.getResponseHeaders().set(k, v));
    ex.sendResponseHeaders(reply.status, bytes.length == 0 ? -1 : bytes.length);
    try (OutputStream out = ex.getResponseBody()) {
      out.write(bytes);
    }
  }

  @Override
  public void close() {
    server.stop(0);
  }
}
