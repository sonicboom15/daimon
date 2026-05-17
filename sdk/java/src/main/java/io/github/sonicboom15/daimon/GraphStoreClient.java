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
 * Client for the {@code /v1/graph/{store}/*} endpoints.
 *
 * <p>Obtain instances via {@link Client#graph(String)}.
 */
public final class GraphStoreClient {

    private final HttpClient http;
    private final String     baseUrl;
    private final String     store;
    private final long       timeoutMs;

    public GraphStoreClient(HttpClient http, String baseUrl, String store, long timeoutMs) {
        this.http      = http;
        this.baseUrl   = baseUrl;
        this.store     = store;
        this.timeoutMs = timeoutMs;
    }

    // =========================================================================
    // addNode
    // =========================================================================

    /**
     * Upserts a graph node.  When {@code id} is non-null and non-empty a PUT
     * is used; otherwise a POST is used and the server assigns the ID.
     *
     * @return the node ID (caller-supplied or server-assigned)
     */
    public String addNode(String id, List<String> labels, Map<String, Object> props) {
        JsonObject body = new JsonObject();

        if (labels != null && !labels.isEmpty()) {
            JsonArray labelsArr = new JsonArray();
            labels.forEach(labelsArr::add);
            body.add("labels", labelsArr);
        }

        if (props != null && !props.isEmpty()) {
            String propsJson = new Gson().toJson(props);
            body.add("props", JsonParser.parseString(propsJson).getAsJsonObject());
        }

        boolean hasCallerId = id != null && !id.isEmpty();
        String url = hasCallerId
                ? baseUrl + "/v1/graph/" + store + "/nodes/" + id
                : baseUrl + "/v1/graph/" + store + "/nodes";

        HttpRequest.Builder reqBuilder = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .header("Content-Type", "application/json");

        HttpRequest req = hasCallerId
                ? reqBuilder.PUT(HttpRequest.BodyPublishers.ofString(body.toString())).build()
                : reqBuilder.POST(HttpRequest.BodyPublishers.ofString(body.toString())).build();

        HttpResponse<String> resp = send(req);
        checkStatus(resp);

        try {
            JsonObject respObj = JsonParser.parseString(resp.body()).getAsJsonObject();
            if (respObj.has("id")) return respObj.get("id").getAsString();
        } catch (Exception ignored) { /* fall through */ }

        return hasCallerId ? id : null;
    }

    // =========================================================================
    // addEdge
    // =========================================================================

    public void addEdge(String from, String to, String type, Map<String, Object> props) {
        JsonObject body = new JsonObject();
        body.addProperty("from", from);
        body.addProperty("to", to);
        body.addProperty("type", type);

        if (props != null && !props.isEmpty()) {
            String propsJson = new Gson().toJson(props);
            body.add("props", JsonParser.parseString(propsJson).getAsJsonObject());
        } else {
            body.add("props", new JsonObject());
        }

        String url = baseUrl + "/v1/graph/" + store + "/edges";
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body.toString()))
                .build();

        checkStatus(send(req));
    }

    // =========================================================================
    // cypher
    // =========================================================================

    /**
     * Executes a Cypher query and returns the rows as a list of maps.
     */
    @SuppressWarnings("unchecked")
    public List<Map<String, Object>> cypher(String query, Map<String, Object> params) {
        JsonObject body = new JsonObject();
        body.addProperty("query", query);

        if (params != null && !params.isEmpty()) {
            String paramsJson = new Gson().toJson(params);
            body.add("params", JsonParser.parseString(paramsJson).getAsJsonObject());
        } else {
            body.add("params", new JsonObject());
        }

        String url = baseUrl + "/v1/graph/" + store + "/cypher";
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body.toString()))
                .build();

        HttpResponse<String> resp = send(req);
        checkStatus(resp);

        List<Map<String, Object>> rows = new ArrayList<>();
        try {
            JsonObject respObj = JsonParser.parseString(resp.body()).getAsJsonObject();
            if (respObj.has("rows")) {
                JsonArray arr = respObj.getAsJsonArray("rows");
                Gson gson = new Gson();
                for (int i = 0; i < arr.size(); i++) {
                    rows.add(gson.fromJson(arr.get(i), Map.class));
                }
            }
        } catch (Exception e) {
            throw new DaimonException("Failed to parse cypher response: " + e.getMessage(), e);
        }
        return rows;
    }

    // =========================================================================
    // deleteNode
    // =========================================================================

    public void deleteNode(String id) {
        String url = baseUrl + "/v1/graph/" + store + "/nodes/" + id;
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
