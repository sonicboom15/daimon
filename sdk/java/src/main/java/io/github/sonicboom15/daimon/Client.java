// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import java.net.http.HttpClient;
import java.time.Duration;

/**
 * Main entry point for the Daimon sidecar SDK.
 *
 * <pre>{@code
 * Client client = new Client();               // defaults to http://localhost:8080
 * String reply = client.chat("assistant", "Hello!");
 * }</pre>
 */
public final class Client {

    private static final String DEFAULT_BASE_URL  = "http://127.0.0.1:3500";
    private static final long   DEFAULT_TIMEOUT_MS = 120_000L;

    private final HttpClient http;
    private final String     baseUrl;
    private final long       timeoutMs;

    // =========================================================================
    // Constructors
    // =========================================================================

    public Client() {
        this(DEFAULT_BASE_URL, DEFAULT_TIMEOUT_MS);
    }

    public Client(String baseUrl) {
        this(baseUrl, DEFAULT_TIMEOUT_MS);
    }

    public Client(String baseUrl, long timeoutMs) {
        this.baseUrl   = baseUrl.endsWith("/") ? baseUrl.substring(0, baseUrl.length() - 1) : baseUrl;
        this.timeoutMs = timeoutMs;
        this.http = HttpClient.newBuilder()
                .connectTimeout(Duration.ofMillis(timeoutMs))
                .build();
    }

    /** Package-private constructor for tests that supply a pre-built HttpClient. */
    Client(HttpClient http, String baseUrl, long timeoutMs) {
        this.http      = http;
        this.baseUrl   = baseUrl.endsWith("/") ? baseUrl.substring(0, baseUrl.length() - 1) : baseUrl;
        this.timeoutMs = timeoutMs;
    }

    // =========================================================================
    // Sub-client factories
    // =========================================================================

    /** Returns an {@link LLMClient} for the {@code "default"} component. */
    public LLMClient llm() {
        return llm("default");
    }

    /** Returns an {@link LLMClient} for the named component. */
    public LLMClient llm(String component) {
        return new LLMClient(http, baseUrl, component, timeoutMs);
    }

    /** Returns a {@link MemoryStoreClient} for the named memory store. */
    public MemoryStoreClient memory(String store) {
        return new MemoryStoreClient(http, baseUrl, store, timeoutMs);
    }

    /** Returns a {@link GraphStoreClient} for the named graph store. */
    public GraphStoreClient graph(String store) {
        return new GraphStoreClient(http, baseUrl, store, timeoutMs);
    }

    // =========================================================================
    // Shorthand helpers
    // =========================================================================

    /**
     * Shorthand for {@code llm(component).chat(prompt)}.
     */
    public String chat(String component, String prompt) {
        return llm(component).chat(prompt);
    }

    /**
     * Shorthand for {@code llm(component).stream(prompt)}.
     */
    public Iterable<String> stream(String component, String prompt) {
        return llm(component).stream(prompt);
    }
}
