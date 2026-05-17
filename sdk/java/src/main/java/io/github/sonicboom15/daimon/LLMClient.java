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
import java.util.Iterator;
import java.util.List;
import java.util.NoSuchElementException;
import java.util.function.Function;
import java.util.stream.Stream;

/**
 * Client for the {@code /v1/converse/{component}} SSE endpoint and the
 * {@code DELETE /v1/sessions/{id}} management endpoint.
 *
 * <p>Obtain instances via {@link Client#llm(String)}.
 */
public final class LLMClient {

    private final HttpClient http;
    private final String     baseUrl;
    private final String     component;
    private final long       timeoutMs;

    public LLMClient(HttpClient http, String baseUrl, String component, long timeoutMs) {
        this.http      = http;
        this.baseUrl   = baseUrl;
        this.component = component;
        this.timeoutMs = timeoutMs;
    }

    // =========================================================================
    // chat() — blocking, returns full response text
    // =========================================================================

    public String chat(String prompt) {
        return chat(List.of(Message.user(prompt)), ChatOptions.defaults());
    }

    public String chat(String prompt, ChatOptions options) {
        return chat(List.of(Message.user(prompt)), options);
    }

    public String chat(List<Message> messages, ChatOptions options) {
        StringBuilder sb = new StringBuilder();
        for (Chunk chunk : converse(messages, options)) {
            if (chunk.isError()) {
                throw new DaimonException(chunk.error());
            }
            if (chunk.isDone()) {
                break;
            }
            if (chunk.isText() && chunk.text() != null) {
                sb.append(chunk.text());
            }
        }
        return sb.toString();
    }

    // =========================================================================
    // stream() — lazy iterable of text fragments
    // =========================================================================

    public Iterable<String> stream(String prompt) {
        return stream(List.of(Message.user(prompt)), ChatOptions.defaults());
    }

    public Iterable<String> stream(String prompt, ChatOptions options) {
        return stream(List.of(Message.user(prompt)), options);
    }

    public Iterable<String> stream(List<Message> messages, ChatOptions options) {
        return () -> new SseIterator<>(openLineStream(messages, options), chunk -> {
            if (chunk.isError()) throw new DaimonException(chunk.error());
            if (chunk.isDone())  return null;          // signals end-of-stream
            if (chunk.isText())  return chunk.text();
            return "";  // skip non-text chunks in text-only mode
        });
    }

    // =========================================================================
    // converse() — lazy iterable of all Chunk types
    // =========================================================================

    public Iterable<Chunk> converse(List<Message> messages, ChatOptions options) {
        return () -> new SseIterator<>(openLineStream(messages, options), chunk -> {
            if (chunk.isDone()) return null;  // signals end-of-stream
            return chunk;
        });
    }

    // =========================================================================
    // clearSession()
    // =========================================================================

    public void clearSession(String sessionId) {
        String url = baseUrl + "/v1/sessions/" + sessionId;
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .DELETE()
                .build();
        try {
            HttpResponse<String> resp = http.send(req, HttpResponse.BodyHandlers.ofString());
            if (resp.statusCode() < 200 || resp.statusCode() >= 300) {
                throw new DaimonException("HTTP " + resp.statusCode() + ": " + resp.body());
            }
        } catch (IOException | InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new DaimonException("clearSession failed: " + e.getMessage(), e);
        }
    }

    // =========================================================================
    // Private helpers
    // =========================================================================

    /** Opens the SSE connection and returns the raw line {@link Stream}. */
    private Stream<String> openLineStream(List<Message> messages, ChatOptions options) {
        String url  = baseUrl + "/v1/converse/" + component;
        String body = buildRequestBody(messages, options);

        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofMillis(timeoutMs))
                .header("Content-Type", "application/json")
                .header("Accept", "text/event-stream")
                .POST(HttpRequest.BodyPublishers.ofString(body))
                .build();
        try {
            HttpResponse<Stream<String>> resp =
                    http.send(req, HttpResponse.BodyHandlers.ofLines());
            if (resp.statusCode() < 200 || resp.statusCode() >= 300) {
                // Drain a small portion to include in the error message.
                String preview = resp.body()
                        .limit(20)
                        .reduce("", (a, b) -> a + b + "\n")
                        .trim();
                resp.body().close();
                throw new DaimonException("HTTP " + resp.statusCode() + ": " + preview);
            }
            return resp.body();
        } catch (DaimonException de) {
            throw de;
        } catch (IOException | InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new DaimonException("converse request failed: " + e.getMessage(), e);
        }
    }

    private String buildRequestBody(List<Message> messages, ChatOptions options) {
        JsonObject body = new JsonObject();

        JsonArray msgsArray = new JsonArray();
        for (Message m : messages) msgsArray.add(m.toJson());
        body.add("messages", msgsArray);

        if (options != null) options.toJsonFields(body);

        return body.toString();
    }

    // =========================================================================
    // SseIterator — generic lazy SSE iterator
    // =========================================================================

    /**
     * Reads lines from an SSE stream, parses each {@code data: ...} line as a
     * {@link Chunk}, maps it through {@code mapper}, and stops when the mapper
     * returns {@code null} (end-of-stream signal).
     *
     * @param <T> the element type yielded by the iterator
     */
    private static final class SseIterator<T> implements Iterator<T> {

        private final Iterator<String>   lines;
        private final Function<Chunk, T> mapper;
        private final Stream<String>     lineStream;

        /** Next buffered value, or {@code null} if we need to fetch one. */
        private T       next;
        /** {@code true} once the stream has reached a terminal chunk. */
        private boolean done;

        SseIterator(Stream<String> lineStream, Function<Chunk, T> mapper) {
            this.lineStream = lineStream;
            this.lines      = lineStream.iterator();
            this.mapper     = mapper;
            advance();
        }

        private void advance() {
            next = null;
            if (done) return;

            while (lines.hasNext()) {
                String line = lines.next();
                if (!line.startsWith("data: ")) continue;

                String payload = line.substring(6).trim();
                if (payload.isEmpty()) continue;

                JsonObject obj;
                try {
                    obj = JsonParser.parseString(payload).getAsJsonObject();
                } catch (Exception ex) {
                    // Malformed JSON — skip this line.
                    continue;
                }

                Chunk chunk = Chunk.fromJson(obj);
                T value = mapper.apply(chunk);

                if (value == null) {
                    // Mapper signals end-of-stream (done or error-terminal).
                    done = true;
                    lineStream.close();
                    return;
                }

                // For the text-only stream we skip empty strings emitted for
                // non-text chunks; keep looking for the next real fragment.
                if (value instanceof String s && s.isEmpty()) {
                    continue;
                }

                next = value;
                return;
            }

            // Underlying stream exhausted.
            done = true;
            lineStream.close();
        }

        @Override
        public boolean hasNext() {
            return next != null;
        }

        @Override
        public T next() {
            if (next == null) throw new NoSuchElementException();
            T current = next;
            advance();
            return current;
        }
    }
}
