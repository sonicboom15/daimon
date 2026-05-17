// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import java.util.List;

/**
 * A single message in a conversation turn.
 *
 * <p>{@code toolCalls} and {@code toolCallId} are only populated for assistant
 * and tool-result messages respectively; they are {@code null} for ordinary
 * user/assistant/system messages.
 */
public record Message(String role, String content, List<ToolCall> toolCalls, String toolCallId) {

    /** Null-safe accessor — always returns a non-null list. */
    @Override
    public List<ToolCall> toolCalls() {
        return toolCalls == null ? List.of() : toolCalls;
    }

    // -------------------------------------------------------------------------
    // Static factories
    // -------------------------------------------------------------------------

    public static Message user(String content) {
        return new Message("user", content, null, null);
    }

    public static Message assistant(String content) {
        return new Message("assistant", content, null, null);
    }

    public static Message system(String content) {
        return new Message("system", content, null, null);
    }

    public static Message tool(String content, String toolCallId) {
        return new Message("tool", content, null, toolCallId);
    }

    // -------------------------------------------------------------------------
    // Serialisation
    // -------------------------------------------------------------------------

    /**
     * Converts this message to the JSON shape expected by the sidecar.
     */
    public JsonObject toJson() {
        JsonObject obj = new JsonObject();
        obj.addProperty("role", role);
        obj.addProperty("content", content != null ? content : "");

        if (toolCallId != null) {
            obj.addProperty("tool_call_id", toolCallId);
        }

        List<ToolCall> calls = toolCalls();
        if (!calls.isEmpty()) {
            JsonArray arr = new JsonArray();
            for (ToolCall tc : calls) {
                JsonObject tcObj = new JsonObject();
                if (tc.id() != null) tcObj.addProperty("id", tc.id());
                if (tc.name() != null) tcObj.addProperty("name", tc.name());
                arr.add(tcObj);
            }
            obj.add("tool_calls", arr);
        }

        return obj;
    }
}
