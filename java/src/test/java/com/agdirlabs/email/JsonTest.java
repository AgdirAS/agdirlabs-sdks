package com.agdirlabs.email;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

class JsonTest {
  @Test
  void roundTripsValues() {
    String text = "{\"s\":\"a\\\"b\\\\c\\n\\u00e9\\u2028\",\"n\":-12,\"d\":1.5e3,\"t\":true,\"f\":false,"
        + "\"z\":null,\"a\":[1,[],{}],\"o\":{\"k\":\"v\"}}";
    Map<String, Object> m = Json.object(Json.parse(text));
    assertEquals("a\"b\\c\né\u2028", m.get("s"));
    assertEquals(-12L, m.get("n"));
    assertEquals(1500.0, m.get("d"));
    assertEquals(true, m.get("t"));
    assertEquals(false, m.get("f"));
    assertNull(m.get("z"));
    assertEquals(Arrays.asList(1L, List.of(), Map.of()), m.get("a"));
    assertEquals(Json.parse(Json.write(m)), m);
  }

  @Test
  void escapesControlCharactersWhenWriting() {
    Map<String, Object> m = new LinkedHashMap<>();
    m.put("x", "tab\tbell\u0007 é \u2028");
    m.put("n", null);
    m.put("d", 2.50);
    assertEquals("{\"x\":\"tab\\tbell\\u0007 é \\u2028\",\"n\":null,\"d\":2.5}", Json.write(m));
  }

  @Test
  void parsesSurrogatePairsAndWhitespace() {
    assertEquals("😀", Json.parse(" \"\\ud83d\\ude00\" "));
    assertEquals("😀", Json.parse("\"😀\""));
  }

  @Test
  void rejectsMalformedInput() {
    for (String bad : List.of("", "{", "[1,]", "{\"a\" 1}", "tru", "01x", "\"\\x\"", "[1] 2", "-", "1.", "\"a")) {
      assertThrows(IllegalArgumentException.class, () -> Json.parse(bad), bad);
    }
  }
}
