package com.agdirlabs.email;

/** Confirmation that an object was deleted. */
public final class Deleted {
  private final String id;

  Deleted(String id) {
    this.id = id;
  }

  public String getId() {
    return id;
  }

  /** Always {@code true}. */
  public boolean isDeleted() {
    return true;
  }

  @Override
  public String toString() {
    return "Deleted{id=" + id + "}";
  }
}
