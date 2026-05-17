// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.*;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Unit tests for {@link LLMClient} backed by an in-process {@link HttpServer}.
 */
class LLMClientTest {

    private HttpServer server;
    private int        port;
    private HttpClient httpClient;

    @BeforeEach
    void startServer() throws IOException {
        server = HttpServer.create(new InetSocketAddress(0), 0);
        port   = server.getAddress().getPort();
        server.start();
        httpClient = HttpClient.newHttpClient();
    }

    @AfterEach
    void stopServer() {
        server.stop(0);
    }

    // -------------------------------------------------------------------------
    // Helpers
    // -------------------------------------------------------------------------

    private LLMClient client(String component) {
        return new LLMClient(httpClient, "http://localhost:" + port, component, 10_000);
    }

    /**
     * Registers an SSE handler on the given path that writes the supplied
     * raw SSE lines (each element is already a full {@code data: …} line
     * without a trailing newline; the handler adds {@code \n\n}).
     */
    private void registerSseHandler(String path, String... sseLines) {
        server.createContext(path, exchange -> {
            exchange.getResponseHeaders().set("Content-Type", "text/event-stream");
            StringBuilder sb = new StringBuilder();
            for (String line : sseLines) sb.append(line).append("\n\n");
            byte[] bytes = sb.toString().getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(200, bytes.length);
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(bytes);
            }
        });
    }

    /** Reads the request body from an exchange as a UTF-8 string. */
    private static String body(HttpExchange exchange) throws IOException {
        try (InputStream is = exchange.getRequestBody()) {
            return new String(is.readAllBytes(), StandardCharsets.UTF_8);
        }
    }

    // -------------------------------------------------------------------------
    // Tests
    // -------------------------------------------------------------------------

    @Test
    void testChat_returnsCollectedText() {
        registerSseHandler("/v1/converse/default",
                "data: {\"type\":\"text\",\"text\":\"Hello\"}",
                "data: {\"type\":\"text\",\"text\":\", world!\"}",
                "data: {\"type\":\"done\"}");

        String result = client("default").chat("hi");
        assertEquals("Hello, world!", result);
    }

    @Test
    void testChat_throwsOnError() {
        registerSseHandler("/v1/converse/default",
                "data: {\"type\":\"error\",\"error\":\"model overloaded\"}");

        DaimonException ex = assertThrows(DaimonException.class,
                () -> client("default").chat("hi"));
        assertTrue(ex.getMessage().contains("model overloaded"));
    }

    @Test
    void testStream_yieldsFragments() {
        registerSseHandler("/v1/converse/default",
                "data: {\"type\":\"text\",\"text\":\"foo\"}",
                "data: {\"type\":\"text\",\"text\":\"bar\"}",
                "data: {\"type\":\"done\"}");

        List<String> fragments = new ArrayList<>();
        for (String s : client("default").stream("hi")) {
            fragments.add(s);
        }
        assertEquals(List.of("foo", "bar"), fragments);
    }

    @Test
    void testStream_unknownComponent_404() {
        server.createContext("/v1/converse/ghost", exchange -> {
            byte[] body = "not found".getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(404, body.length);
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(body);
            }
        });

        DaimonException ex = assertThrows(DaimonException.class, () -> {
            // Iterating forces the lazy stream to open the connection.
            client("ghost").stream("hi").iterator().hasNext();
        });
        assertTrue(ex.getMessage().startsWith("HTTP 404"), "expected HTTP 404 prefix, got: " + ex.getMessage());
    }

    @Test
    void testClearSession_callsDelete() throws IOException {
        AtomicReference<String> method = new AtomicReference<>();
        server.createContext("/v1/sessions/abc123", exchange -> {
            method.set(exchange.getRequestMethod());
            exchange.sendResponseHeaders(204, -1);
            exchange.getResponseBody().close();
        });

        client("default").clearSession("abc123");
        assertEquals("DELETE", method.get());
    }

    @Test
    void testChat_withOptions_sendsParams() throws IOException {
        AtomicReference<String> captured = new AtomicReference<>();

        server.createContext("/v1/converse/default", exchange -> {
            captured.set(body(exchange));
            exchange.getResponseHeaders().set("Content-Type", "text/event-stream");
            byte[] resp = "data: {\"type\":\"done\"}\n\n".getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(200, resp.length);
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(resp);
            }
        });

        ChatOptions opts = ChatOptions.builder()
                .temperature(0.5)
                .maxTokens(256)
                .seed(99)
                .build();
        client("default").chat("hello", opts);

        assertNotNull(captured.get(), "request body was not captured");
        JsonObject body = JsonParser.parseString(captured.get()).getAsJsonObject();
        assertEquals(0.5,  body.get("temperature").getAsDouble(), 1e-9);
        assertEquals(256,  body.get("max_tokens").getAsInt());
        assertEquals(99,   body.get("seed").getAsInt());
    }

    @Test
    void testChat_namedComponent_usesCorrectPath() throws IOException {
        AtomicReference<String> capturedPath = new AtomicReference<>();

        server.createContext("/v1/converse/mybot", exchange -> {
            capturedPath.set(exchange.getRequestURI().getPath());
            exchange.getResponseHeaders().set("Content-Type", "text/event-stream");
            byte[] resp = "data: {\"type\":\"done\"}\n\n".getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(200, resp.length);
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(resp);
            }
        });

        client("mybot").chat("hi");
        assertEquals("/v1/converse/mybot", capturedPath.get());
    }

    @Test
    void testConverse_yieldsAllChunkTypes() {
        registerSseHandler("/v1/converse/default",
                "data: {\"type\":\"text\",\"text\":\"thinking...\"}",
                "data: {\"type\":\"tool_call\",\"tool_call\":{\"id\":\"tc1\",\"name\":\"search\",\"input\":{\"q\":\"java\"}}}",
                "data: {\"type\":\"done\"}");

        List<Chunk> chunks = new ArrayList<>();
        for (Chunk c : client("default").converse(List.of(Message.user("go")), ChatOptions.defaults())) {
            chunks.add(c);
        }

        assertEquals(2, chunks.size());
        assertTrue(chunks.get(0).isText());
        assertEquals("thinking...", chunks.get(0).text());
        assertTrue(chunks.get(1).isToolCall());
        assertNotNull(chunks.get(1).toolCall());
        assertEquals("search", chunks.get(1).toolCall().name());
    }
}
