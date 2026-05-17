// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.JsonArray;
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
 * Unit tests for {@link GraphStoreClient} backed by an in-process {@link HttpServer}.
 */
class GraphStoreClientTest {

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

    private GraphStoreClient client(String store) {
        return new GraphStoreClient(httpClient, "http://localhost:" + port, store, 10_000);
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
    // addNode (PUT — caller-supplied ID)
    // -------------------------------------------------------------------------

    @Test
    void testAddNode_callerSuppliedId_usesPUT() throws IOException {
        AtomicReference<String> capturedMethod = new AtomicReference<>();
        AtomicReference<String> capturedBody   = new AtomicReference<>();

        server.createContext("/v1/graph/kg/nodes/alice", exchange -> {
            capturedMethod.set(exchange.getRequestMethod());
            capturedBody.set(body(exchange));
            jsonResponse(exchange, 200, "{\"id\":\"alice\"}");
        });

        String id = client("kg").addNode("alice", List.of("Person"), Map.of("name", "Alice"));

        assertEquals("PUT", capturedMethod.get());
        assertEquals("alice", id);

        JsonObject sent = JsonParser.parseString(capturedBody.get()).getAsJsonObject();
        JsonArray labels = sent.getAsJsonArray("labels");
        assertEquals("Person", labels.get(0).getAsString());
        assertEquals("Alice", sent.getAsJsonObject("props").get("name").getAsString());
    }

    // -------------------------------------------------------------------------
    // addNode (POST — server-assigned ID)
    // -------------------------------------------------------------------------

    @Test
    void testAddNode_noId_usesPOST() throws IOException {
        AtomicReference<String> capturedMethod = new AtomicReference<>();

        server.createContext("/v1/graph/kg/nodes", exchange -> {
            capturedMethod.set(exchange.getRequestMethod());
            jsonResponse(exchange, 200, "{\"id\":\"auto-node-1\"}");
        });

        String id = client("kg").addNode(null, List.of("Event"), null);

        assertEquals("POST", capturedMethod.get());
        assertEquals("auto-node-1", id);
    }

    // -------------------------------------------------------------------------
    // addEdge
    // -------------------------------------------------------------------------

    @Test
    void testAddEdge_sendsPOST_withCorrectBody() throws IOException {
        AtomicReference<String> capturedMethod = new AtomicReference<>();
        AtomicReference<String> capturedBody   = new AtomicReference<>();

        server.createContext("/v1/graph/kg/edges", exchange -> {
            capturedMethod.set(exchange.getRequestMethod());
            capturedBody.set(body(exchange));
            exchange.sendResponseHeaders(204, -1);
            exchange.getResponseBody().close();
        });

        client("kg").addEdge("alice", "bob", "KNOWS", Map.of("since", "2020"));

        assertEquals("POST", capturedMethod.get());
        JsonObject sent = JsonParser.parseString(capturedBody.get()).getAsJsonObject();
        assertEquals("alice",  sent.get("from").getAsString());
        assertEquals("bob",    sent.get("to").getAsString());
        assertEquals("KNOWS",  sent.get("type").getAsString());
        assertEquals("2020",   sent.getAsJsonObject("props").get("since").getAsString());
    }

    @Test
    void testAddEdge_noProps_sendsEmptyPropsObject() throws IOException {
        AtomicReference<String> capturedBody = new AtomicReference<>();

        server.createContext("/v1/graph/kg/edges", exchange -> {
            capturedBody.set(body(exchange));
            exchange.sendResponseHeaders(204, -1);
            exchange.getResponseBody().close();
        });

        client("kg").addEdge("x", "y", "RELATED", null);

        JsonObject sent = JsonParser.parseString(capturedBody.get()).getAsJsonObject();
        assertTrue(sent.has("props"), "expected props field");
        assertTrue(sent.getAsJsonObject("props").entrySet().isEmpty(), "expected empty props");
    }

    // -------------------------------------------------------------------------
    // cypher
    // -------------------------------------------------------------------------

    @Test
    void testCypher_returnsRows() throws IOException {
        AtomicReference<String> capturedBody = new AtomicReference<>();

        server.createContext("/v1/graph/kg/cypher", exchange -> {
            capturedBody.set(body(exchange));
            String resp = "{\"rows\":["
                    + "{\"a.name\":\"Alice\",\"b.name\":\"Bob\"},"
                    + "{\"a.name\":\"Alice\",\"b.name\":\"Carol\"}"
                    + "]}";
            jsonResponse(exchange, 200, resp);
        });

        List<Map<String, Object>> rows = client("kg")
                .cypher("MATCH (a:Person)-[:KNOWS]->(b) RETURN a.name, b.name", null);

        JsonObject sent = JsonParser.parseString(capturedBody.get()).getAsJsonObject();
        assertEquals("MATCH (a:Person)-[:KNOWS]->(b) RETURN a.name, b.name",
                sent.get("query").getAsString());

        assertEquals(2, rows.size());
        assertEquals("Alice", rows.get(0).get("a.name"));
        assertEquals("Bob",   rows.get(0).get("b.name"));
        assertEquals("Carol", rows.get(1).get("b.name"));
    }

    @Test
    void testCypher_emptyRows_returnsEmptyList() throws IOException {
        server.createContext("/v1/graph/kg/cypher", exchange ->
                jsonResponse(exchange, 200, "{\"rows\":[]}"));

        List<Map<String, Object>> rows = client("kg").cypher("MATCH (n) RETURN n LIMIT 0", null);
        assertTrue(rows.isEmpty());
    }

    @Test
    void testCypher_withParams_sendsParams() throws IOException {
        AtomicReference<String> capturedBody = new AtomicReference<>();

        server.createContext("/v1/graph/kg/cypher", exchange -> {
            capturedBody.set(body(exchange));
            jsonResponse(exchange, 200, "{\"rows\":[{\"n.name\":\"Alice\"}]}");
        });

        client("kg").cypher("MATCH (n {name: $name}) RETURN n.name", Map.of("name", "Alice"));

        JsonObject sent = JsonParser.parseString(capturedBody.get()).getAsJsonObject();
        assertEquals("Alice", sent.getAsJsonObject("params").get("name").getAsString());
    }

    // -------------------------------------------------------------------------
    // deleteNode
    // -------------------------------------------------------------------------

    @Test
    void testDeleteNode_callsDELETE() throws IOException {
        AtomicReference<String> capturedMethod = new AtomicReference<>();

        server.createContext("/v1/graph/kg/nodes/alice", exchange -> {
            capturedMethod.set(exchange.getRequestMethod());
            exchange.sendResponseHeaders(204, -1);
            exchange.getResponseBody().close();
        });

        client("kg").deleteNode("alice");
        assertEquals("DELETE", capturedMethod.get());
    }

    // -------------------------------------------------------------------------
    // error handling
    // -------------------------------------------------------------------------

    @Test
    void testAddNode_httpError_throwsDaimonException() throws IOException {
        server.createContext("/v1/graph/kg/nodes/bad", exchange ->
                jsonResponse(exchange, 500, "{\"error\":\"backend error\"}"));

        DaimonException ex = assertThrows(DaimonException.class,
                () -> client("kg").addNode("bad", null, null));
        assertTrue(ex.getMessage().startsWith("HTTP 500"));
    }

    @Test
    void testCypher_unknownStore_throwsDaimonException() throws IOException {
        server.createContext("/v1/graph/ghost/cypher", exchange ->
                jsonResponse(exchange, 404, "{\"error\":\"store not found\"}"));

        DaimonException ex = assertThrows(DaimonException.class,
                () -> client("ghost").cypher("MATCH (n) RETURN n", null));
        assertTrue(ex.getMessage().startsWith("HTTP 404"));
    }

    @Test
    void testAddEdge_serverError_throwsDaimonException() throws IOException {
        server.createContext("/v1/graph/kg/edges", exchange ->
                jsonResponse(exchange, 400, "{\"error\":\"invalid relationship type\"}"));

        DaimonException ex = assertThrows(DaimonException.class,
                () -> client("kg").addEdge("a", "b", "INVALID TYPE", null));
        assertTrue(ex.getMessage().startsWith("HTTP 400"));
    }
}
