package config

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

type RetryConfig struct {
	MaxRetries        *int     `json:"max_retries,omitempty" jsonschema:"minimum=0,maximum=20"`
	InitialDelayMS    *int     `json:"initial_delay_ms,omitempty" jsonschema:"minimum=0,maximum=3600000"`
	BackoffMultiplier *float64 `json:"backoff_multiplier,omitempty" jsonschema:"minimum=1,maximum=100"`
	MaxDelayMS        *int     `json:"max_delay_ms,omitempty" jsonschema:"minimum=1,maximum=3600000"`
	Jitter            *string  `json:"jitter,omitempty" jsonschema:"enum=full,enum=none"`
	MaxElapsedMS      *int     `json:"max_elapsed_ms,omitempty" jsonschema:"minimum=1,maximum=86400000"`
	StreamPolicy      *string  `json:"stream_policy,omitempty" jsonschema:"enum=safe_step,enum=before_output"`
}

type ResolvedRetry struct {
	MaxRetries        int
	InitialDelay      time.Duration
	BackoffMultiplier float64
	MaxDelay          time.Duration
	Jitter            string
	MaxElapsed        time.Duration
	StreamPolicy      string
}

func (r *RetryConfig) UnmarshalJSON(data []byte) error {
	type fields RetryConfig
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for key := range raw {
		switch key {
		case "max_retries", "initial_delay_ms", "backoff_multiplier", "max_delay_ms", "jitter", "max_elapsed_ms", "stream_policy":
		default:
			return fmt.Errorf("unknown retry option %q", key)
		}
	}
	return json.Unmarshal(data, (*fields)(r))
}

func (r *RetryConfig) validate() error {
	if r == nil {
		return nil
	}
	switch {
	case r.InitialDelayMS != nil && (*r.InitialDelayMS < 0 || *r.InitialDelayMS > 3_600_000):
		return fmt.Errorf("initial_delay_ms must be between 0 and 3600000")
	case r.MaxDelayMS != nil && (*r.MaxDelayMS <= 0 || *r.MaxDelayMS > 3_600_000):
		return fmt.Errorf("max_delay_ms must be between 1 and 3600000")
	case r.MaxElapsedMS != nil && (*r.MaxElapsedMS <= 0 || *r.MaxElapsedMS > 86_400_000):
		return fmt.Errorf("max_elapsed_ms must be between 1 and 86400000")
	case r.MaxRetries != nil && (*r.MaxRetries < 0 || *r.MaxRetries > 20):
		return fmt.Errorf("max_retries must be between 0 and 20")
	case r.BackoffMultiplier != nil && (math.IsNaN(*r.BackoffMultiplier) || math.IsInf(*r.BackoffMultiplier, 0) || *r.BackoffMultiplier < 1 || *r.BackoffMultiplier > 100):
		return fmt.Errorf("backoff_multiplier must be finite and between 1 and 100")
	case r.Jitter != nil && *r.Jitter != "none" && *r.Jitter != "full":
		return fmt.Errorf("jitter must be full or none")
	case r.StreamPolicy != nil && *r.StreamPolicy != "safe_step" && *r.StreamPolicy != "before_output":
		return fmt.Errorf("stream_policy must be safe_step or before_output")
	}
	return nil
}

func ResolveRetry(global, provider *RetryConfig) (ResolvedRetry, error) {
	policy := ResolvedRetry{
		MaxRetries: 2, InitialDelay: 2 * time.Second, BackoffMultiplier: 2,
		MaxDelay: 30 * time.Second, Jitter: "full", MaxElapsed: 2 * time.Minute,
		StreamPolicy: "safe_step",
	}
	for _, layer := range []*RetryConfig{global, provider} {
		if layer == nil {
			continue
		}
		if err := layer.validate(); err != nil {
			return ResolvedRetry{}, err
		}
		if layer.MaxRetries != nil {
			policy.MaxRetries = *layer.MaxRetries
		}
		if layer.InitialDelayMS != nil {
			policy.InitialDelay = time.Duration(*layer.InitialDelayMS) * time.Millisecond
		}
		if layer.BackoffMultiplier != nil {
			policy.BackoffMultiplier = *layer.BackoffMultiplier
		}
		if layer.MaxDelayMS != nil {
			policy.MaxDelay = time.Duration(*layer.MaxDelayMS) * time.Millisecond
		}
		if layer.Jitter != nil {
			policy.Jitter = *layer.Jitter
		}
		if layer.MaxElapsedMS != nil {
			policy.MaxElapsed = time.Duration(*layer.MaxElapsedMS) * time.Millisecond
		}
		if layer.StreamPolicy != nil {
			policy.StreamPolicy = *layer.StreamPolicy
		}
	}
	if policy.MaxRetries < 0 || policy.MaxRetries > 20 || policy.InitialDelay < 0 || policy.InitialDelay > time.Hour || policy.MaxDelay <= 0 || policy.MaxDelay > time.Hour || policy.InitialDelay > policy.MaxDelay || math.IsNaN(policy.BackoffMultiplier) || math.IsInf(policy.BackoffMultiplier, 0) || policy.BackoffMultiplier < 1 || policy.BackoffMultiplier > 100 || policy.MaxElapsed <= 0 || policy.MaxElapsed > 24*time.Hour || (policy.Jitter != "full" && policy.Jitter != "none") || (policy.StreamPolicy != "safe_step" && policy.StreamPolicy != "before_output") {
		return ResolvedRetry{}, fmt.Errorf("invalid retry configuration")
	}
	return policy, nil
}

func (c *Config) ValidateRetry() error {
	var global *RetryConfig
	if c.Options != nil {
		global = c.Options.Retry
	}
	if _, err := ResolveRetry(global, nil); err != nil {
		return fmt.Errorf("options.retry: %w", err)
	}
	if c.Providers != nil {
		for provider := range c.Providers.Seq() {
			if _, err := ResolveRetry(global, provider.Retry); err != nil {
				return fmt.Errorf("providers.%s.retry: %w", provider.ID, err)
			}
		}
	}
	return nil
}
