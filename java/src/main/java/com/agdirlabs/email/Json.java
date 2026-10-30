package com.agdirlabs.email;

import java.math.BigDecimal;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Minimal JSON reader/writer (RFC 8259) so the SDK has no runtime dependencies.
 *
 * <p>Objects parse to {@code LinkedHashMap<String, Object>}, arrays to {@code List<Object>}, numbers to
 * {@code Long} (integers that fit) or {@code Double}, plus {@code String}, {@code Boolean} and {@code null}.
 */
final class Json {
  private Json() {}

  // ---- writing ----

  static String write(Object value) {
    StringBuilder out = new StringBuilder();
    write(value, out);
    return out.toString();
  }

  private static void write(Object value, StringBuilder out) {
    if (value == null) {
      out.append("null");
    } else if (value instanceof String) {
      writeString((String) value, out);
    } else if (value instanceof Boolean) {
      out.append(value);
    } else if (value instanceof Double || value instanceof Float) {
      double d = ((Number) value).doubleValue();
      if (Double.isNaN(d) || Double.isInfinite(d)) throw new IllegalArgumentException("JSON cannot encode " + d);
      out.append(Double.toString(d)); // always valid JSON for finite values, e.g. 1500.0 or 1.0E20
    } else if (value instanceof Number) {
      out.append(value);
    } else if (value instanceof Map) {
      out.append('{');
      boolean first = true;
      for (Map.Entry<?, ?> e : ((Map<?, ?>) value).entrySet()) {
        if (!first) out.append(',');
        first = false;
        writeString(String.valueOf(e.getKey()), out);
        out.append(':');
        write(e.getValue(), out);
      }
      out.append('}');
    } else if (value instanceof Collection) {
      out.append('[');
      boolean first = true;
      for (Object item : (Collection<?>) value) {
        if (!first) out.append(',');
        first = false;
        write(item, out);
      }
      out.append(']');
    } else {
      throw new IllegalArgumentException("JSON cannot encode " + value.getClass().getName());
    }
  }

  private static void writeString(String s, StringBuilder out) {
    out.append('"');
    for (int i = 0; i < s.length(); i++) {
      char c = s.charAt(i);
      switch (c) {
        case '"':
          out.append("\\\"");
          break;
        case '\\':
          out.append("\\\\");
          break;
        case '\n':
          out.append("\\n");
          break;
        case '\r':
          out.append("\\r");
          break;
        case '\t':
          out.append("\\t");
          break;
        case '\b':
          out.append("\\b");
          break;
        case '\f':
          out.append("\\f");
          break;
        default:
          // Escape controls, plus U+2028/2029 so the output is also safe inside JavaScript.
          if (c < 0x20 || c == ' ' || c == ' ') {
            out.append(String.format("\\u%04x", (int) c));
          } else {
            out.append(c);
          }
      }
    }
    out.append('"');
  }

  // ---- reading ----

  /** Parses a JSON document. Throws {@link IllegalArgumentException} on malformed input. */
  static Object parse(String text) {
    Parser p = new Parser(text);
    p.skipWhitespace();
    Object value = p.readValue(0);
    p.skipWhitespace();
    if (p.pos != text.length()) throw p.error("Unexpected trailing characters");
    return value;
  }

  private static final class Parser {
    private static final int MAX_DEPTH = 512;
    private final String s;
    private int pos;

    Parser(String s) {
      this.s = s;
    }

    IllegalArgumentException error(String message) {
      return new IllegalArgumentException(message + " at position " + pos);
    }

    void skipWhitespace() {
      while (pos < s.length()) {
        char c = s.charAt(pos);
        if (c == ' ' || c == '\t' || c == '\n' || c == '\r') pos++;
        else break;
      }
    }

    Object readValue(int depth) {
      if (depth > MAX_DEPTH) throw error("JSON nested too deeply");
      if (pos >= s.length()) throw error("Unexpected end of JSON");
      char c = s.charAt(pos);
      switch (c) {
        case '{':
          return readObject(depth);
        case '[':
          return readArray(depth);
        case '"':
          return readString();
        case 't':
          expectWord("true");
          return Boolean.TRUE;
        case 'f':
          expectWord("false");
          return Boolean.FALSE;
        case 'n':
          expectWord("null");
          return null;
        default:
          if (c == '-' || (c >= '0' && c <= '9')) return readNumber();
          throw error("Unexpected character '" + c + "'");
      }
    }

    private void expectWord(String word) {
      if (!s.startsWith(word, pos)) throw error("Expected " + word);
      pos += word.length();
    }

    private Map<String, Object> readObject(int depth) {
      Map<String, Object> map = new LinkedHashMap<>();
      pos++; // {
      skipWhitespace();
      if (peek() == '}') {
        pos++;
        return map;
      }
      while (true) {
        skipWhitespace();
        if (peek() != '"') throw error("Expected a string key");
        String key = readString();
        skipWhitespace();
        if (peek() != ':') throw error("Expected ':'");
        pos++;
        skipWhitespace();
        map.put(key, readValue(depth + 1));
        skipWhitespace();
        char c = peek();
        pos++;
        if (c == '}') return map;
        if (c != ',') throw error("Expected ',' or '}'");
      }
    }

    private List<Object> readArray(int depth) {
      List<Object> list = new ArrayList<>();
      pos++; // [
      skipWhitespace();
      if (peek() == ']') {
        pos++;
        return list;
      }
      while (true) {
        skipWhitespace();
        list.add(readValue(depth + 1));
        skipWhitespace();
        char c = peek();
        pos++;
        if (c == ']') return list;
        if (c != ',') throw error("Expected ',' or ']'");
      }
    }

    private char peek() {
      if (pos >= s.length()) throw error("Unexpected end of JSON");
      return s.charAt(pos);
    }

    private String readString() {
      pos++; // opening quote
      StringBuilder out = new StringBuilder();
      while (true) {
        char c = peek();
        pos++;
        if (c == '"') return out.toString();
        if (c < 0x20) throw error("Unescaped control character in string");
        if (c != '\\') {
          out.append(c);
          continue;
        }
        char e = peek();
        pos++;
        switch (e) {
          case '"':
          case '\\':
          case '/':
            out.append(e);
            break;
          case 'b':
            out.append('\b');
            break;
          case 'f':
            out.append('\f');
            break;
          case 'n':
            out.append('\n');
            break;
          case 'r':
            out.append('\r');
            break;
          case 't':
            out.append('\t');
            break;
          case 'u':
            if (pos + 4 > s.length()) throw error("Bad unicode escape");
            try {
              out.append((char) Integer.parseInt(s.substring(pos, pos + 4), 16));
            } catch (NumberFormatException ex) {
              throw error("Bad unicode escape");
            }
            pos += 4;
            break;
          default:
            throw error("Bad escape '\\" + e + "'");
        }
      }
    }

    private Object readNumber() {
      int start = pos;
      if (s.charAt(pos) == '-') pos++;
      boolean integral = true;
      int digits = digits();
      if (digits == 0) throw error("Bad number");
      if (pos < s.length() && s.charAt(pos) == '.') {
        integral = false;
        pos++;
        if (digits() == 0) throw error("Bad number");
      }
      if (pos < s.length() && (s.charAt(pos) == 'e' || s.charAt(pos) == 'E')) {
        integral = false;
        pos++;
        if (pos < s.length() && (s.charAt(pos) == '+' || s.charAt(pos) == '-')) pos++;
        if (digits() == 0) throw error("Bad number");
      }
      String text = s.substring(start, pos);
      if (integral) {
        try {
          return Long.parseLong(text);
        } catch (NumberFormatException tooBig) {
          return new BigDecimal(text).doubleValue();
        }
      }
      return Double.parseDouble(text);
    }

    private int digits() {
      int start = pos;
      while (pos < s.length() && s.charAt(pos) >= '0' && s.charAt(pos) <= '9') pos++;
      return pos - start;
    }
  }

  // ---- typed access helpers for mapping API responses ----

  @SuppressWarnings("unchecked")
  static Map<String, Object> object(Object value) {
    if (value instanceof Map) return (Map<String, Object>) value;
    return Collections.emptyMap();
  }

  @SuppressWarnings("unchecked")
  static List<Object> array(Object value) {
    if (value instanceof List) return (List<Object>) value;
    return Collections.emptyList();
  }

  static String string(Map<String, Object> m, String key) {
    Object v = m.get(key);
    return v == null ? null : String.valueOf(v);
  }

  static boolean bool(Map<String, Object> m, String key) {
    return Boolean.TRUE.equals(m.get(key));
  }

  static Integer integer(Map<String, Object> m, String key) {
    Object v = m.get(key);
    return v instanceof Number ? ((Number) v).intValue() : null;
  }
}
