package com.agdirlabs.email;

import static com.agdirlabs.email.Http.seg;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.concurrent.CompletableFuture;

/** Sending domains: {@code agdir.domains()}. */
public final class Domains {
  private final Http http;

  Domains(Http http) {
    this.http = http;
  }

  /** Adds a sending domain. Publish the returned records in DNS, then call {@link #verify}. */
  public DomainWithRecords create(String name) {
    return Http.await(createAsync(name, null));
  }

  /** Adds a sending domain in a region (default {@code eu-north-1}). */
  public DomainWithRecords create(String name, String region) {
    return Http.await(createAsync(name, region));
  }

  public CompletableFuture<DomainWithRecords> createAsync(String name) {
    return createAsync(name, null);
  }

  public CompletableFuture<DomainWithRecords> createAsync(String name, String region) {
    Map<String, Object> body = new LinkedHashMap<>();
    body.put("name", name);
    if (region != null) body.put("region", region);
    return http.send(Http.Call.post("/domains").body(body), Wire::domainWithRecords);
  }

  public DomainWithRecords get(String id) {
    return Http.await(getAsync(id));
  }

  public CompletableFuture<DomainWithRecords> getAsync(String id) {
    return http.send(Http.Call.get("/domains/" + seg(id)), Wire::domainWithRecords);
  }

  /** The first page of domains. */
  public Page<Domain> list() {
    return list(ListOptions.defaults());
  }

  public Page<Domain> list(ListOptions options) {
    return Http.await(listAsync(options));
  }

  public CompletableFuture<Page<Domain>> listAsync(ListOptions options) {
    ListOptions o = options == null ? ListOptions.defaults() : options;
    Http.Call call = Http.Call.get("/domains").query("limit", o.getLimit()).query("after", o.getAfter());
    return http.send(call, Wire.page(Wire::domain));
  }

  /** Every domain, fetching further pages as you iterate. */
  public PagedIterable<Domain> listAll() {
    return listAll(ListOptions.defaults());
  }

  public PagedIterable<Domain> listAll(ListOptions options) {
    return new PagedIterable<>(this::list, options);
  }

  /** Starts a DNS check now instead of waiting for the next automatic one. Returns the domain's id. */
  public String verify(String id) {
    return Http.await(verifyAsync(id));
  }

  public CompletableFuture<String> verifyAsync(String id) {
    return http.send(Http.Call.post("/domains/" + seg(id) + "/verify").retryable(true), Wire::id);
  }

  public Deleted delete(String id) {
    return Http.await(deleteAsync(id));
  }

  public CompletableFuture<Deleted> deleteAsync(String id) {
    return http.send(Http.Call.delete("/domains/" + seg(id)), body -> new Deleted(Wire.id(body)));
  }
}
