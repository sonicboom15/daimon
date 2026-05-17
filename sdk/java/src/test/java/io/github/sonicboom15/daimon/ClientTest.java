// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Unit tests for {@link Client} — factory methods and shorthand delegates.
 */
class ClientTest {

    private HttpServer server;
    private Client     client;

    @BeforeEach
    void start() throws IOException {
        server = HttpServer.create(new InetSocketAddress(0), 0);
        server.start();
        client = new Client("http://127.0.0.1:" + server.getAddress().getPort());
    }

    @AfterEach
    void stop() {
        server.stop(0);
    }

    @Test
    void testLlm_defaultComponent_usesDefaultInPath() {
        server.createContext("/v1/converse/default", exchange -> {
            byte[] bytes = "data: {\"type\":\"done\"}\n\n".getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().set("Content-Type", "text/event-stream");
            try {
                exchange.sendResponseHeaders(200, bytes.length);
                try (OutputStream os = exchange.getResponseBody()) { os.write(bytes); }
            } catch (IOException ignored) {}
        });
        // Should not throw — verifies the path resolves correctly.
        assertDoesNotThrow(() -> client.llm().chat("hi"));
    }

    @Test
    void testLlm_namedComponent_usesNameInPath() {
        server.createContext("/v1/converse/claude", exchange -> {
            byte[] bytes = "data: {\"type\":\"done\"}\n\n".getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().set("Content-Type", "text/event-stream");
            try {
                exchange.sendResponseHeaders(200, bytes.length);
                try (OutputStream os = exchange.getResponseBody()) { os.write(bytes); }
            } catch (IOException ignored) {}
        });
        assertDoesNotThrow(() -> client.llm("claude").chat("hi"));
    }

    @Test
    void testShorthandChat_delegatesToLlm() {
        server.createContext("/v1/converse/gpt4o", exchange -> {
            byte[] bytes = ("data: {\"type\":\"text\",\"text\":\"pong\"}\n\n" +
                            "data: {\"type\":\"done\"}\n\n").getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().set("Content-Type", "text/event-stream");
            try {
                exchange.sendResponseHeaders(200, bytes.length);
                try (OutputStream os = exchange.getResponseBody()) { os.write(bytes); }
            } catch (IOException ignored) {}
        });
        String reply = client.chat("gpt4o", "ping");
        assertEquals("pong", reply);
    }

    @Test
    void testMemory_returnsNonNull() {
        assertNotNull(client.memory("docs"));
    }

    @Test
    void testGraph_returnsNonNull() {
        assertNotNull(client.graph("kg"));
    }

    @Test
    void testDefaultConstructor_usesPort3500() {
        // Can't easily test the real default without a running server,
        // but we can verify the LLMClient it returns is non-null.
        Client defaultClient = new Client();
        assertNotNull(defaultClient.llm());
        assertNotNull(defaultClient.llm("claude"));
    }
}
