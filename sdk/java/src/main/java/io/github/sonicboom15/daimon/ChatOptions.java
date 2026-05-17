// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import java.util.List;

/**
 * Immutable bag of optional parameters forwarded to the sidecar on each
 * {@code /v1/converse} call.
 *
 * <p>Obtain an instance via {@link #builder()} or {@link #defaults()}.
 */
public final class ChatOptions {

    private final String      model;
    private final String      system;
    private final String      sessionId;
    private final Integer     maxTokens;
    private final Double      temperature;
    private final Double      topP;
    private final Integer     topK;
    private final Integer     seed;
    private final Double      frequencyPenalty;
    private final Double      presencePenalty;
    private final List<Tool>  tools;

    private ChatOptions(Builder b) {
        this.model            = b.model;
        this.system           = b.system;
        this.sessionId        = b.sessionId;
        this.maxTokens        = b.maxTokens;
        this.temperature      = b.temperature;
        this.topP             = b.topP;
        this.topK             = b.topK;
        this.seed             = b.seed;
        this.frequencyPenalty = b.frequencyPenalty;
        this.presencePenalty  = b.presencePenalty;
        this.tools            = b.tools;
    }

    // -------------------------------------------------------------------------
    // Accessors
    // -------------------------------------------------------------------------

    public String      model()            { return model; }
    public String      system()           { return system; }
    public String      sessionId()        { return sessionId; }
    public Integer     maxTokens()        { return maxTokens; }
    public Double      temperature()      { return temperature; }
    public Double      topP()             { return topP; }
    public Integer     topK()             { return topK; }
    public Integer     seed()             { return seed; }
    public Double      frequencyPenalty() { return frequencyPenalty; }
    public Double      presencePenalty()  { return presencePenalty; }
    public List<Tool>  tools()            { return tools; }

    // -------------------------------------------------------------------------
    // Factory helpers
    // -------------------------------------------------------------------------

    /** Returns a new {@link Builder}. */
    public static Builder builder() { return new Builder(); }

    /** Returns a {@link ChatOptions} with every field set to {@code null}. */
    public static ChatOptions defaults() { return new Builder().build(); }

    // -------------------------------------------------------------------------
    // Serialisation
    // -------------------------------------------------------------------------

    /**
     * Adds every non-null field of this options object as a property of the
     * supplied {@link JsonObject} in-place.
     */
    public void toJsonFields(JsonObject obj) {
        if (model            != null) obj.addProperty("model",             model);
        if (system           != null) obj.addProperty("system",            system);
        if (sessionId        != null) obj.addProperty("session_id",        sessionId);
        if (maxTokens        != null) obj.addProperty("max_tokens",        maxTokens);
        if (temperature      != null) obj.addProperty("temperature",       temperature);
        if (topP             != null) obj.addProperty("top_p",             topP);
        if (topK             != null) obj.addProperty("top_k",             topK);
        if (seed             != null) obj.addProperty("seed",              seed);
        if (frequencyPenalty != null) obj.addProperty("frequency_penalty", frequencyPenalty);
        if (presencePenalty  != null) obj.addProperty("presence_penalty",  presencePenalty);
        if (tools != null && !tools.isEmpty()) {
            JsonArray arr = new JsonArray();
            for (Tool t : tools) arr.add(t.toJson());
            obj.add("tools", arr);
        }
    }

    // -------------------------------------------------------------------------
    // Builder
    // -------------------------------------------------------------------------

    public static final class Builder {

        private String      model;
        private String      system;
        private String      sessionId;
        private Integer     maxTokens;
        private Double      temperature;
        private Double      topP;
        private Integer     topK;
        private Integer     seed;
        private Double      frequencyPenalty;
        private Double      presencePenalty;
        private List<Tool>  tools;

        private Builder() {}

        public Builder model(String v)            { this.model            = v; return this; }
        public Builder system(String v)           { this.system           = v; return this; }
        public Builder sessionId(String v)        { this.sessionId        = v; return this; }
        public Builder maxTokens(Integer v)       { this.maxTokens        = v; return this; }
        public Builder temperature(Double v)      { this.temperature      = v; return this; }
        public Builder topP(Double v)             { this.topP             = v; return this; }
        public Builder topK(Integer v)            { this.topK             = v; return this; }
        public Builder seed(Integer v)            { this.seed             = v; return this; }
        public Builder frequencyPenalty(Double v) { this.frequencyPenalty = v; return this; }
        public Builder presencePenalty(Double v)  { this.presencePenalty  = v; return this; }
        public Builder tools(List<Tool> v)        { this.tools            = v; return this; }

        public ChatOptions build() { return new ChatOptions(this); }
    }
}
