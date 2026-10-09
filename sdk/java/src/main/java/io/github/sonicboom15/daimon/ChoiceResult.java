// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import java.util.HashMap;
import java.util.Map;

/**
 * Result of a decision choose evaluation returned by {@code POST /v1/decision/{model}/choose}.
 */
public record ChoiceResult(String selected, int index, Map<String, Double> probabilities) {

    /**
     * Deserialises a {@link ChoiceResult} from a JSON object.
     */
    public static ChoiceResult fromJson(JsonObject obj) {
        String selected = obj.has("selected") ? obj.get("selected").getAsString() : "";
        int index       = obj.has("index")    ? obj.get("index").getAsInt()       : 0;

        Map<String, Double> probabilities = new HashMap<>();
        if (obj.has("probabilities") && obj.get("probabilities").isJsonObject()) {
            JsonObject probsObj = obj.getAsJsonObject("probabilities");
            for (Map.Entry<String, JsonElement> entry : probsObj.entrySet()) {
                if (entry.getValue().isJsonPrimitive()) {
                    probabilities.put(entry.getKey(), entry.getValue().getAsDouble());
                }
            }
        }

        return new ChoiceResult(selected, index, probabilities);
    }
}
