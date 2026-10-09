// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package decision defines the Model interface and data types for calibrated
// discriminative choice selection and statement verification components.
package decision

import "context"

// ChoiceRequest requests selecting the single best match among discrete candidates.
type ChoiceRequest struct {
	State    string   `json:"state"`
	Question string   `json:"question"`
	Choices  []string `json:"choices"`
}

// ChoiceResponse contains the selected choice, its candidate index, and probability distribution.
type ChoiceResponse struct {
	Selected      string             `json:"selected"`
	Index         int                `json:"index"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// VerifyRequest asks whether a given statement is supported by the context state.
type VerifyRequest struct {
	State     string `json:"state"`
	Statement string `json:"statement"`
}

// VerifyResponse returns the calibrated probability and boolean decision flag.
type VerifyResponse struct {
	Probability float64 `json:"probability"`
	Supported   bool    `json:"supported"`
}

// Model evaluates discrete decisions and assertion verifications against context.
type Model interface {
	Choose(ctx context.Context, req ChoiceRequest) (*ChoiceResponse, error)
	Verify(ctx context.Context, req VerifyRequest) (*VerifyResponse, error)
}

// Config is handed to every Decision Factory at construction time.
type Config struct {
	Metadata map[string]string
}

// Factory creates a Model from a Config.
type Factory func(cfg Config) (Model, error)

