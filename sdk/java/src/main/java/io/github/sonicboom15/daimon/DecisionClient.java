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
import java.util.List;

/**
 * Client for the {@code /v1/decision/{model}/*} endpoints.
 *
 * <p>Obtain instances via {@link Client#decision(String)}.
 */
public final class DecisionClient {

    private final HttpClient http;
    private final String     baseUrl;
    private final String     model;
    private final long       timeoutMs;

    public DecisionClient(HttpClient http, String baseUrl, String model, long timeoutMs) {
        this.http      = http;
        this.baseUrl   = baseUrl;
        this.model     = model;
        this.timeoutMs = timeoutMs;
    }

    /**
     * Evaluates candidates and chooses the best option.
     */
    public ChoiceResult choose(String question, List<String> choices) {
        return choose(question, choices, null);
    }

    /**
     * Evaluates candidates and chooses the best option given a context state.
     */
    public ChoiceResult choose(String question, List<String> choices, String state) {
        JsonObject body = new JsonObject();
        body.addProperty("question", question);
        JsonArray arr = new JsonArray();
        for (String c : choices) {
            arr.add(c);
        }
        body.add("choices", arr);
        if (state != null && !state.isEmpty()) {
            body.addProperty("state", state);
        }

        String url = baseUrl + "/v1/decision/" + model + "/choose";
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body.toString()))
                .build();

        HttpResponse<String> resp = send(req);
        checkStatus(resp);

        try {
            JsonObject respObj = JsonParser.parseString(resp.body()).getAsJsonObject();
            return ChoiceResult.fromJson(respObj);
        } catch (Exception e) {
            throw new DaimonException("Failed to parse decision choose response: " + e.getMessage(), e);
        }
    }

    /**
     * Evaluates the truth probability of a statement.
     */
    public VerifyResult verify(String statement) {
        return verify(statement, null);
    }

    /**
     * Evaluates the truth probability of a statement given a context state.
     */
    public VerifyResult verify(String statement, String state) {
        JsonObject body = new JsonObject();
        body.addProperty("statement", statement);
        if (state != null && !state.isEmpty()) {
            body.addProperty("state", state);
        }

        String url = baseUrl + "/v1/decision/" + model + "/verify";
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body.toString()))
                .build();

        HttpResponse<String> resp = send(req);
        checkStatus(resp);

        try {
            JsonObject respObj = JsonParser.parseString(resp.body()).getAsJsonObject();
            return VerifyResult.fromJson(respObj);
        } catch (Exception e) {
            throw new DaimonException("Failed to parse decision verify response: " + e.getMessage(), e);
        }
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
