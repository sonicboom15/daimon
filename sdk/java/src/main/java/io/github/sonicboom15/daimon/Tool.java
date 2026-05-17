// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.Gson;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import java.util.Map;

/**
 * Describes a tool that the model may call during a conversation.
 */
public record Tool(String name, String description, Map<String, Object> inputSchema) {

    /**
     * Serialises this tool to a {@link JsonObject} using the field names
     * expected by the Daimon sidecar ({@code name}, {@code description},
     * {@code input_schema}).
     */
    public JsonObject toJson() {
        JsonObject obj = new JsonObject();
        obj.addProperty("name", name);
        obj.addProperty("description", description);
        if (inputSchema != null) {
            String schemaJson = new Gson().toJson(inputSchema);
            obj.add("input_schema", JsonParser.parseString(schemaJson).getAsJsonObject());
        } else {
            obj.add("input_schema", new JsonObject());
        }
        return obj;
    }
}
