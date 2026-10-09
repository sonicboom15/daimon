// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	nerhttp "github.com/sonicboom15/daimon/internal/components/ner/http"
	"github.com/sonicboom15/daimon/internal/ner"
)

func TestNERHTTPDriver(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/extract" {
			http.NotFound(w, r)
			return
		}

		var req ner.ExtractRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := ner.ExtractResponse{
			Entities: []ner.Entity{
				{
					Text:       "knee pain",
					Label:      "symptom",
					Start:      0,
					End:        9,
					Confidence: 0.92,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	model, err := nerhttp.New(ner.Config{
		Metadata: map[string]string{
			"base_url":          ts.URL,
			"default_labels":    "disease,symptom",
			"default_threshold": "0.6",
		},
	})
	if err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	res, err := model.Extract(context.Background(), ner.ExtractRequest{
		Text: "knee pain",
	})
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}

	if len(res.Entities) != 1 || res.Entities[0].Label != "symptom" {
		t.Fatalf("unexpected response: %+v", res)
	}
}

