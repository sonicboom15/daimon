// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.JsonObject;

/**
 * Result of a decision verify assertion returned by {@code POST /v1/decision/{model}/verify}.
 */
public record VerifyResult(double probability, boolean supported) {

    /**
     * Deserialises a {@link VerifyResult} from a JSON object.
     */
    public static VerifyResult fromJson(JsonObject obj) {
        double probability = obj.has("probability") ? obj.get("probability").getAsDouble() : 0.0;
        boolean supported  = obj.has("supported")   ? obj.get("supported").getAsBoolean()   : false;
        return new VerifyResult(probability, supported);
    }
}
