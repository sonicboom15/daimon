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
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Unit tests for {@link MemoryStoreClient} backed by an in-process {@link HttpServer}.
 */
class MemoryStoreClientTest {

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

    private MemoryStoreClient client(String store) {
        return new MemoryStoreClient(httpClient, "http://localhost:" + port, store, 10_000);
    }

    private void jsonResponse(HttpExchange exchange, int status, String json) throws IOException {
        byte[] bytes = json.getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", "application/json");
        exchange.sendResponseHeaders(status, bytes.length);
        try (OutputStream os = exchange.getResponseBody()) {
            os.write(bytes);
        }
    }

    private static String body(HttpExchange exchange) throws IOException {
        try (InputStream is = exchange.getRequestBody()) {
            return new String(is.readAllBytes(), StandardCharsets.UTF_8);
        }
    }

    // -------------------------------------------------------------------------
    // upsert (POST — server-assigned ID)
    // -------------------------------------------------------------------------

    @Test
    void testUpsert_serverAssignedId_usesPOST() throws IOException {
        AtomicReference<String> capturedMethod = new AtomicReference<>();
        AtomicReference<String> capturedBody   = new AtomicReference<>();

        server.createContext("/v1/memory/docs", exchange -> {
            capturedMethod.set(exchange.getRequestMethod());
            capturedBody.set(body(exchange));
            jsonResponse(exchange, 200, "{\"id\":\"server-id-1\"}");
        });

        String id = client("docs").upsert("hello world");

        assertEquals("POST", capturedMethod.get());
        assertEquals("server-id-1", id);
        JsonObject sent = JsonParser.parseString(capturedBody.get()).getAsJsonObject();
        assertEquals("hello world", sent.get("content").getAsString());
    }

    // -------------------------------------------------------------------------
    // upsert (PUT — caller-supplied ID)
    // -------------------------------------------------------------------------

    @Test
    void testUpsert_callerSuppliedId_usesPUT() throws IOException {
        AtomicReference<String> capturedMethod = new AtomicReference<>();
        AtomicReference<String> capturedBody   = new AtomicReference<>();

        server.createContext("/v1/memory/docs/doc42", exchange -> {
            capturedMethod.set(exchange.getRequestMethod());
            capturedBody.set(body(exchange));
            jsonResponse(exchange, 200, "{\"id\":\"doc42\"}");
        });

        String id = client("docs").upsert("my content", "doc42", null);

        assertEquals("PUT", capturedMethod.get());
        assertEquals("doc42", id);
        JsonObject sent = JsonParser.parseString(capturedBody.get()).getAsJsonObject();
        assertEquals("my content", sent.get("content").getAsString());
    }

    @Test
    void testUpsert_withMetadata_sendsMetadata() throws IOException {
        AtomicReference<String> capturedBody = new AtomicReference<>();

        server.createContext("/v1/memory/docs/meta-doc", exchange -> {
            capturedBody.set(body(exchange));
            jsonResponse(exchange, 200, "{\"id\":\"meta-doc\"}");
        });

        client("docs").upsert("content", "meta-doc", Map.of("source", "wiki", "lang", "en"));

        JsonObject sent = JsonParser.parseString(capturedBody.get()).getAsJsonObject();
        assertTrue(sent.has("metadata"), "expected metadata in body");
        JsonObject meta = sent.getAsJsonObject("metadata");
        assertEquals("wiki", meta.get("source").getAsString());
        assertEquals("en",   meta.get("lang").getAsString());
    }

    // -------------------------------------------------------------------------
    // query
    // -------------------------------------------------------------------------

    @Test
    void testQuery_returnsResults() throws IOException {
        AtomicReference<String> capturedBody = new AtomicReference<>();

        server.createContext("/v1/memory/docs/query", exchange -> {
            capturedBody.set(body(exchange));
            String resp = "{\"results\":["
                    + "{\"id\":\"d1\",\"content\":\"Paris is in France\",\"score\":0.9},"
                    + "{\"id\":\"d2\",\"content\":\"Berlin is in Germany\",\"score\":0.6}"
                    + "]}";
            jsonResponse(exchange, 200, resp);
        });

        List<MemoryResult> results = client("docs").query("European capitals", 2);

        JsonObject sent = JsonParser.parseString(capturedBody.get()).getAsJsonObject();
        assertEquals("European capitals", sent.get("query").getAsString());
        assertEquals(2, sent.get("top_k").getAsInt());

        assertEquals(2, results.size());
        assertEquals("d1",                 results.get(0).id());
        assertEquals("Paris is in France", results.get(0).content());
        assertEquals(0.9,                  results.get(0).score(), 1e-9);
        assertEquals("d2",                 results.get(1).id());
    }

    @Test
    void testQuery_emptyResults_returnsEmptyList() throws IOException {
        server.createContext("/v1/memory/docs/query", exchange ->
                jsonResponse(exchange, 200, "{\"results\":[]}"));

        List<MemoryResult> results = client("docs").query("nothing", 5);
        assertTrue(results.isEmpty());
    }

    // -------------------------------------------------------------------------
    // delete
    // -------------------------------------------------------------------------

    @Test
    void testDelete_callsDELETE() throws IOException {
        AtomicReference<String> capturedMethod = new AtomicReference<>();

        server.createContext("/v1/memory/docs/doc42", exchange -> {
            capturedMethod.set(exchange.getRequestMethod());
            exchange.sendResponseHeaders(204, -1);
            exchange.getResponseBody().close();
        });

        client("docs").delete("doc42");
        assertEquals("DELETE", capturedMethod.get());
    }

    // -------------------------------------------------------------------------
    // error handling
    // -------------------------------------------------------------------------

    @Test
    void testUpsert_httpError_throwsDaimonException() throws IOException {
        server.createContext("/v1/memory/docs", exchange ->
                jsonResponse(exchange, 500, "{\"error\":\"internal server error\"}"));

        DaimonException ex = assertThrows(DaimonException.class,
                () -> client("docs").upsert("content"));
        assertTrue(ex.getMessage().startsWith("HTTP 500"));
    }

    @Test
    void testQuery_unknownStore_throwsDaimonException() throws IOException {
        server.createContext("/v1/memory/ghost/query", exchange ->
                jsonResponse(exchange, 404, "{\"error\":\"store not found\"}"));

        DaimonException ex = assertThrows(DaimonException.class,
                () -> client("ghost").query("test", 5));
        assertTrue(ex.getMessage().startsWith("HTTP 404"));
    }

    @Test
    void testDelete_unknownId_throwsDaimonException() throws IOException {
        server.createContext("/v1/memory/docs/missing", exchange ->
                jsonResponse(exchange, 404, "{\"error\":\"not found\"}"));

        DaimonException ex = assertThrows(DaimonException.class,
                () -> client("docs").delete("missing"));
        assertTrue(ex.getMessage().startsWith("HTTP 404"));
    }
}
