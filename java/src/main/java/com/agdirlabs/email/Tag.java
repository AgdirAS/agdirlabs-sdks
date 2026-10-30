package com.agdirlabs.email;

import java.util.Objects;

/** A name/value pair attached to an email and echoed back in webhook events. ASCII letters, digits, {@code _} and {@code -} only. */
public final class Tag {
  private final String name;
  private final String value;

  private Tag(String name, String value) {
    this.name = Objects.requireNonNull(name, "name");
    this.value = Objects.requireNonNull(value, "value");
  }

  public static Tag of(String name, String value) {
    return new Tag(name, value);
  }

  public String getName() {
    return name;
  }

  public String getValue() {
    return value;
  }

  @Override
  public boolean equals(Object o) {
    return o instanceof Tag && name.equals(((Tag) o).name) && value.equals(((Tag) o).value);
  }

  @Override
  public int hashCode() {
    return Objects.hash(name, value);
  }

  @Override
  public String toString() {
    return name + "=" + value;
  }
}
