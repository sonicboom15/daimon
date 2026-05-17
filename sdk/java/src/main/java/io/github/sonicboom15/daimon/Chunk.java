// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.JsonObject;

/**
 * A single SSE chunk emitted by the {@code /v1/converse/{component}} endpoint.
 *
 * <p>Exactly one of {@code text}, {@code toolCall}, or {@code error} will be
 * non-null depending on the value of {@code type}.
 */
public record Chunk(String type, String text, ToolCall toolCall, String error) {

    // -------------------------------------------------------------------------
    // Boolean helpers
    // -------------------------------------------------------------------------

    public boolean isText()     { return "text".equals(type); }
    public boolean isDone()     { return "done".equals(type); }
    public boolean isError()    { return "error".equals(type); }
    public boolean isToolCall() { return "tool_call".equals(type); }

    // -------------------------------------------------------------------------
    // Deserialisation
    // -------------------------------------------------------------------------

    /**
     * Parses a {@link Chunk} from a raw SSE JSON payload.
     */
    public static Chunk fromJson(JsonObject obj) {
        String type = obj.has("type") ? obj.get("type").getAsString() : null;

        String text = null;
        ToolCall toolCall = null;
        String error = null;

        if ("text".equals(type) && obj.has("text")) {
            text = obj.get("text").getAsString();
        } else if ("tool_call".equals(type) && obj.has("tool_call")) {
            toolCall = ToolCall.fromJson(obj.getAsJsonObject("tool_call"));
        } else if ("error".equals(type) && obj.has("error")) {
            error = obj.get("error").getAsString();
        }

        return new Chunk(type, text, toolCall, error);
    }
}
