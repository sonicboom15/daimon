// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

class NERClientTest {

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

    private NERClient client(String model) {
        return new NERClient(httpClient, "http://localhost:" + port, model, 10_000);
    }

    private void jsonResponse(HttpExchange exchange, int status, String json) throws IOException {
        byte[] bytes = json.getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", "application/json");
        exchange.sendResponseHeaders(status, bytes.length);
        try (OutputStream os = exchange.getResponseBody()) {
            os.write(bytes);
        }
    }

    private String readBody(HttpExchange exchange) throws IOException {
        try (InputStream is = exchange.getRequestBody()) {
            return new String(is.readAllBytes(), StandardCharsets.UTF_8);
        }
    }

    @Test
    void testExtract_simpleText() {
        server.createContext("/v1/ner/clinical/extract", exchange -> {
            try {
                jsonResponse(exchange, 200, """
                    {
                      "entities": [
                        {
                          "text": "aspirin",
                          "label": "medication",
                          "start": 0,
                          "end": 7,
                          "confidence": 0.98
                        }
                      ]
                    }
                    """);
            } catch (Exception ignored) {}
        });

        List<Entity> entities = client("clinical").extract("aspirin 100mg");
        assertEquals(1, entities.size());
        assertEquals("aspirin", entities.get(0).text());
        assertEquals("medication", entities.get(0).label());
        assertEquals(0, entities.get(0).start());
        assertEquals(7, entities.get(0).end());
        assertEquals(0.98, entities.get(0).confidence(), 0.001);
    }

    @Test
    void testExtract_withLabelsAndThreshold() {
        AtomicReference<String> capturedBody = new AtomicReference<>();
        server.createContext("/v1/ner/clinical/extract", exchange -> {
            try {
                capturedBody.set(readBody(exchange));
                jsonResponse(exchange, 200, "{\"entities\":[]}");
            } catch (Exception ignored) {}
        });

        List<Entity> entities = client("clinical").extract("aspirin 100mg", List.of("medication", "dose"), 0.85);
        assertNotNull(entities);
        assertTrue(entities.isEmpty());

        JsonObject req = JsonParser.parseString(capturedBody.get()).getAsJsonObject();
        assertEquals("aspirin 100mg", req.get("text").getAsString());
        assertEquals(2, req.getAsJsonArray("labels").size());
        assertEquals(0.85, req.get("threshold").getAsDouble(), 0.001);
    }

    @Test
    void testExtract_httpError_throwsDaimonException() {
        server.createContext("/v1/ner/clinical/extract", exchange -> {
            try {
                jsonResponse(exchange, 404, "{\"error\":\"model not found\"}");
            } catch (Exception ignored) {}
        });

        assertThrows(DaimonException.class, () -> client("clinical").extract("test"));
    }
}
