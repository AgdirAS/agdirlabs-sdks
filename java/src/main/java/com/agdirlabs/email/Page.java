package com.agdirlabs.email;

import java.util.Iterator;
import java.util.List;

/** One page of a list call. Iterate it directly, or use {@code listAll()} to walk every page. */
public final class Page<T> implements Iterable<T> {
  private final List<T> data;
  private final boolean hasMore;
  private final String nextCursor;

  Page(List<T> data, boolean hasMore, String nextCursor) {
    this.data = List.copyOf(data);
    this.hasMore = hasMore;
    this.nextCursor = nextCursor;
  }

  public List<T> getData() {
    return data;
  }

  public boolean hasMore() {
    return hasMore;
  }

  /** Pass to {@link ListOptions#after} to fetch the next page; {@code null} on the last page. */
  public String getNextCursor() {
    return nextCursor;
  }

  @Override
  public Iterator<T> iterator() {
    return data.iterator();
  }

  @Override
  public String toString() {
    return "Page{data=" + data + ", hasMore=" + hasMore + ", nextCursor=" + nextCursor + "}";
  }
}
