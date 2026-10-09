// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.JsonObject;

/**
 * A single extracted entity span returned by {@code POST /v1/ner/{model}/extract}.
 */
public record Entity(String text, String label, int start, int end, double confidence) {

    /**
     * Deserialises an {@link Entity} from a JSON object.
     */
    public static Entity fromJson(JsonObject obj) {
        String text       = obj.has("text")       ? obj.get("text").getAsString()       : "";
        String label      = obj.has("label")      ? obj.get("label").getAsString()      : "";
        int start         = obj.has("start")      ? obj.get("start").getAsInt()         : 0;
        int end           = obj.has("end")        ? obj.get("end").getAsInt()           : 0;
        double confidence = obj.has("confidence") ? obj.get("confidence").getAsDouble() : 0.0;
        return new Entity(text, label, start, end, confidence);
    }
}
