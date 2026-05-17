// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.Gson;
import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * Client for the {@code /v1/memory/{store}/*} endpoints.
 *
 * <p>Obtain instances via {@link Client#memory(String)}.
 */
public final class MemoryStoreClient {

    private final HttpClient http;
    private final String     baseUrl;
    private final String     store;
    private final long       timeoutMs;

    public MemoryStoreClient(HttpClient http, String baseUrl, String store, long timeoutMs) {
        this.http      = http;
        this.baseUrl   = baseUrl;
        this.store     = store;
        this.timeoutMs = timeoutMs;
    }

    // =========================================================================
    // upsert — server-assigned ID
    // =========================================================================

    /**
     * POSTs a new document; the server assigns the ID.
     *
     * @return the server-assigned document ID
     */
    public String upsert(String content) {
        return upsert(content, null, null);
    }

    // =========================================================================
    // upsert — caller-supplied ID
    // =========================================================================

    /**
     * Upserts a document.  When {@code id} is non-null and non-empty a PUT is
     * used (caller-supplied ID); otherwise a POST is used (server-assigned ID).
     *
     * @return the document ID (either the supplied {@code id} or the one
     *         assigned by the server)
     */
    public String upsert(String content, String id, Map<String, String> metadata) {
        JsonObject body = new JsonObject();
        body.addProperty("content", content);
        if (metadata != null && !metadata.isEmpty()) {
            String metaJson = new Gson().toJson(metadata);
            body.add("metadata", JsonParser.parseString(metaJson).getAsJsonObject());
        }

        boolean hasCallerId = id != null && !id.isEmpty();
        String url = hasCallerId
                ? baseUrl + "/v1/memory/" + store + "/" + id
                : baseUrl + "/v1/memory/" + store;

        HttpRequest.Builder reqBuilder = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .header("Content-Type", "application/json");

        HttpRequest req = hasCallerId
                ? reqBuilder.PUT(HttpRequest.BodyPublishers.ofString(body.toString())).build()
                : reqBuilder.POST(HttpRequest.BodyPublishers.ofString(body.toString())).build();

        HttpResponse<String> resp = send(req);
        checkStatus(resp);

        // Extract the returned ID from the response body.
        try {
            JsonObject respObj = JsonParser.parseString(resp.body()).getAsJsonObject();
            if (respObj.has("id")) return respObj.get("id").getAsString();
        } catch (Exception ignored) { /* fall through */ }

        return hasCallerId ? id : null;
    }

    // =========================================================================
    // query
    // =========================================================================

    /**
     * Performs a semantic query against the memory store.
     */
    public List<MemoryResult> query(String query, int topK) {
        JsonObject body = new JsonObject();
        body.addProperty("query", query);
        body.addProperty("top_k", topK);

        String url = baseUrl + "/v1/memory/" + store + "/query";
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body.toString()))
                .build();

        HttpResponse<String> resp = send(req);
        checkStatus(resp);

        List<MemoryResult> results = new ArrayList<>();
        try {
            JsonObject respObj = JsonParser.parseString(resp.body()).getAsJsonObject();
            if (respObj.has("results")) {
                JsonArray arr = respObj.getAsJsonArray("results");
                for (int i = 0; i < arr.size(); i++) {
                    results.add(MemoryResult.fromJson(arr.get(i).getAsJsonObject()));
                }
            }
        } catch (Exception e) {
            throw new DaimonException("Failed to parse query response: " + e.getMessage(), e);
        }
        return results;
    }

    // =========================================================================
    // delete
    // =========================================================================

    public void delete(String id) {
        String url = baseUrl + "/v1/memory/" + store + "/" + id;
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .DELETE()
                .build();
        checkStatus(send(req));
    }

    // =========================================================================
    // Private helpers
    // =========================================================================

    private HttpResponse<String> send(HttpRequest req) {
        try {
            return http.send(req, HttpResponse.BodyHandlers.ofString());
        } catch (IOException | InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new DaimonException("Request failed: " + e.getMessage(), e);
        }
    }

    private void checkStatus(HttpResponse<String> resp) {
        if (resp.statusCode() < 200 || resp.statusCode() >= 300) {
            throw new DaimonException("HTTP " + resp.statusCode() + ": " + resp.body());
        }
    }
}
