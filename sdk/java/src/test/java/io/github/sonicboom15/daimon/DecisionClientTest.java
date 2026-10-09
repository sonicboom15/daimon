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

class DecisionClientTest {

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

    private DecisionClient client(String model) {
        return new DecisionClient(httpClient, "http://localhost:" + port, model, 10_000);
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
    void testChoose_success() {
        server.createContext("/v1/decision/verifier/choose", exchange -> {
            try {
                jsonResponse(exchange, 200, """
                    {
                      "selected": "opt-A",
                      "index": 0,
                      "probabilities": {
                        "opt-A": 0.85,
                        "opt-B": 0.15
                      }
                    }
                    """);
            } catch (Exception ignored) {}
        });

        ChoiceResult result = client("verifier").choose("Which one?", List.of("opt-A", "opt-B"), "context");
        assertEquals("opt-A", result.selected());
        assertEquals(0, result.index());
        assertEquals(0.85, result.probabilities().get("opt-A"), 0.001);
        assertEquals(0.15, result.probabilities().get("opt-B"), 0.001);
    }

    @Test
    void testVerify_success() {
        server.createContext("/v1/decision/verifier/verify", exchange -> {
            try {
                jsonResponse(exchange, 200, """
                    {
                      "probability": 0.94,
                      "supported": true
                    }
                    """);
            } catch (Exception ignored) {}
        });

        VerifyResult result = client("verifier").verify("Patient is febrile.", "T: 39C");
        assertEquals(0.94, result.probability(), 0.001);
        assertTrue(result.supported());
    }

    @Test
    void testChoose_httpError_throwsDaimonException() {
        server.createContext("/v1/decision/verifier/choose", exchange -> {
            try {
                jsonResponse(exchange, 500, "{\"error\":\"internal error\"}");
            } catch (Exception ignored) {}
        });

        assertThrows(DaimonException.class, () -> client("verifier").choose("Q", List.of("A", "B")));
    }
}
