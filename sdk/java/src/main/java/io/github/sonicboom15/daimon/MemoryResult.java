// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.Gson;
import com.google.gson.JsonObject;
import java.util.Map;

/**
 * A single result returned by {@code POST /v1/memory/{store}/query}.
 */
public record MemoryResult(String id, String content, double score, Map<String, String> metadata) {

    /**
     * Deserialises a {@link MemoryResult} from one element of the
     * {@code results} array in the query response.
     */
    @SuppressWarnings("unchecked")
    public static MemoryResult fromJson(JsonObject obj) {
        String id      = obj.has("id")      ? obj.get("id").getAsString()      : null;
        String content = obj.has("content") ? obj.get("content").getAsString() : null;
        double score   = obj.has("score")   ? obj.get("score").getAsDouble()   : 0.0;

        Map<String, String> metadata = null;
        if (obj.has("metadata") && !obj.get("metadata").isJsonNull()) {
            metadata = new Gson().fromJson(obj.get("metadata"), Map.class);
        }

        return new MemoryResult(id, content, score, metadata);
    }
}
