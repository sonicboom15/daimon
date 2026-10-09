// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

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

/**
 * Client for the {@code /v1/ner/{model}/*} endpoints.
 *
 * <p>Obtain instances via {@link Client#ner(String)}.
 */
public final class NERClient {

    private final HttpClient http;
    private final String     baseUrl;
    private final String     model;
    private final long       timeoutMs;

    public NERClient(HttpClient http, String baseUrl, String model, long timeoutMs) {
        this.http      = http;
        this.baseUrl   = baseUrl;
        this.model     = model;
        this.timeoutMs = timeoutMs;
    }

    /**
     * Extracts entities and spans from unstructured text with default labels and threshold.
     */
    public List<Entity> extract(String text) {
        return extract(text, null, null);
    }

    /**
     * Extracts entities and spans from unstructured text.
     *
     * @param text      unstructured text to analyze
     * @param labels    optional entity labels to extract (null to use component defaults)
     * @param threshold optional confidence threshold (null to use component default)
     * @return list of extracted {@link Entity} objects
     */
    public List<Entity> extract(String text, List<String> labels, Double threshold) {
        JsonObject body = new JsonObject();
        body.addProperty("text", text);
        if (labels != null && !labels.isEmpty()) {
            JsonArray arr = new JsonArray();
            for (String l : labels) {
                arr.add(l);
            }
            body.add("labels", arr);
        }
        if (threshold != null) {
            body.addProperty("threshold", threshold);
        }

        String url = baseUrl + "/v1/ner/" + model + "/extract";
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body.toString()))
                .build();

        HttpResponse<String> resp = send(req);
        checkStatus(resp);

        List<Entity> entities = new ArrayList<>();
        try {
            JsonObject respObj = JsonParser.parseString(resp.body()).getAsJsonObject();
            if (respObj.has("entities")) {
                JsonArray arr = respObj.getAsJsonArray("entities");
                for (int i = 0; i < arr.size(); i++) {
                    entities.add(Entity.fromJson(arr.get(i).getAsJsonObject()));
                }
            }
        } catch (Exception e) {
            throw new DaimonException("Failed to parse NER extract response: " + e.getMessage(), e);
        }
        return entities;
    }

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
