package com.agdirlabs.email;

import java.util.Iterator;
import java.util.NoSuchElementException;
import java.util.Spliterator;
import java.util.Spliterators;
import java.util.function.Function;
import java.util.stream.Stream;
import java.util.stream.StreamSupport;

/**
 * Every item of a list endpoint, fetching further pages lazily as you iterate. Each call to {@link #iterator()}
 * or {@link #stream()} starts again from the first page.
 *
 * <pre>{@code
 * for (Domain d : agdir.domains().listAll()) System.out.println(d.getName());
 * agdir.webhooks().listAll().stream().filter(w -> w.getStatus() == WebhookStatus.DISABLED).forEach(...);
 * }</pre>
 *
 * <p>Requests are made synchronously by the iterating thread; failures surface as {@link AgdirEmailException}.
 */
public final class PagedIterable<T> implements Iterable<T> {
  private final Function<ListOptions, Page<T>> fetchPage;
  private final ListOptions options;

  PagedIterable(Function<ListOptions, Page<T>> fetchPage, ListOptions options) {
    this.fetchPage = fetchPage;
    this.options = options == null ? ListOptions.defaults() : options;
  }

  /** A sequential, lazily paging stream. */
  public Stream<T> stream() {
    return StreamSupport.stream(Spliterators.spliteratorUnknownSize(iterator(), Spliterator.ORDERED), false);
  }

  @Override
  public Iterator<T> iterator() {
    return new Iterator<T>() {
      private Iterator<T> current;
      private String cursor = options.getAfter();
      private boolean last;

      @Override
      public boolean hasNext() {
        while (current == null || !current.hasNext()) {
          if (last) return false;
          Page<T> page = fetchPage.apply(options.after(cursor));
          current = page.getData().iterator();
          cursor = page.getNextCursor();
          last = cursor == null;
        }
        return true;
      }

      @Override
      public T next() {
        if (!hasNext()) throw new NoSuchElementException();
        return current.next();
      }
    };
  }
}
