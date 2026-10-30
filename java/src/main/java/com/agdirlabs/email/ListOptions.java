package com.agdirlabs.email;

/**
 * Paging options for list calls: {@code ListOptions.limit(50).after(cursor)}. Immutable.
 */
public final class ListOptions {
  private static final ListOptions DEFAULTS = new ListOptions(null, null);

  private final Integer limit;
  private final String after;

  private ListOptions(Integer limit, String after) {
    if (limit != null && (limit < 1 || limit > 100)) throw new IllegalArgumentException("limit must be 1-100");
    this.limit = limit;
    this.after = after;
  }

  /** Server defaults: first page, default page size. */
  public static ListOptions defaults() {
    return DEFAULTS;
  }

  /** Page size, 1–100. */
  public static ListOptions limit(int limit) {
    return new ListOptions(limit, null);
  }

  /** Start after this cursor (the id of the last item of the previous page). */
  public static ListOptions startingAfter(String cursor) {
    return new ListOptions(null, cursor);
  }

  public ListOptions withLimit(int limit) {
    return new ListOptions(limit, after);
  }

  /** Start after this cursor (the id of the last item of the previous page). */
  public ListOptions after(String cursor) {
    return new ListOptions(limit, cursor);
  }

  /** May be {@code null}. */
  public Integer getLimit() {
    return limit;
  }

  /** May be {@code null}. */
  public String getAfter() {
    return after;
  }
}
