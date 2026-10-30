package com.agdirlabs.email;

import java.util.List;

/** Result of {@link Emails#sendBatch}. */
public final class BatchResult {
  private final List<SendResult> data;
  private final List<BatchError> errors;

  BatchResult(List<SendResult> data, List<BatchError> errors) {
    this.data = List.copyOf(data);
    this.errors = List.copyOf(errors);
  }

  /** Accepted emails. In strict mode, one per input in order. */
  public List<SendResult> getData() {
    return data;
  }

  /** Permissive mode only: emails that were not accepted, by input index. Empty otherwise. */
  public List<BatchError> getErrors() {
    return errors;
  }

  @Override
  public String toString() {
    return "BatchResult{data=" + data + ", errors=" + errors + "}";
  }

  /** An email in a permissive batch that was not accepted. */
  public static final class BatchError {
    private final int index;
    private final String message;

    BatchError(int index, String message) {
      this.index = index;
      this.message = message;
    }

    /** Position of the email in the submitted list (0-based). */
    public int getIndex() {
      return index;
    }

    public String getMessage() {
      return message;
    }

    @Override
    public String toString() {
      return "BatchError{index=" + index + ", message=" + message + "}";
    }
  }
}
