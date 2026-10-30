package com.agdirlabs.email;

import java.util.Objects;

/** An accepted email. */
public final class SendResult {
  private final String id;

  SendResult(String id) {
    this.id = id;
  }

  /** The email's id: pass it to {@link Emails#get} or {@link Emails#cancel}. */
  public String getId() {
    return id;
  }

  @Override
  public boolean equals(Object o) {
    return o instanceof SendResult && Objects.equals(id, ((SendResult) o).id);
  }

  @Override
  public int hashCode() {
    return Objects.hashCode(id);
  }

  @Override
  public String toString() {
    return "SendResult{id=" + id + "}";
  }
}
