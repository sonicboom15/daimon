// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.JsonObject;
import com.google.gson.Gson;
import java.util.Map;

/**
 * Represents a tool call emitted by the model during a converse stream.
 */
public record ToolCall(String id, String name, Map<String, Object> input) {

    /**
     * Deserialises a {@code ToolCall} from the {@code tool_call} object found
     * inside a chunk's JSON payload.
     */
    @SuppressWarnings("unchecked")
    public static ToolCall fromJson(JsonObject obj) {
        String id   = obj.has("id")   ? obj.get("id").getAsString()   : null;
        String name = obj.has("name") ? obj.get("name").getAsString() : null;
        Map<String, Object> input = null;
        if (obj.has("input") && !obj.get("input").isJsonNull()) {
            input = new Gson().fromJson(obj.get("input"), Map.class);
        }
        return new ToolCall(id, name, input);
    }
}
