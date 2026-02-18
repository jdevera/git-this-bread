package llmadvice

import (
	"context"
	"errors"
	"os"
)

// Provider defines the interface for LLM providers
type Provider interface {
	// Name returns the provider name
	Name() string
	// Model returns the model being used
	Model() string
	// GenerateAdvice sends a prompt to the LLM and returns advice strings
	GenerateAdvice(ctx context.Context, prompt string) ([]string, error)
}

// ProviderType represents supported LLM providers
type ProviderType string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
)

var (
	ErrNoAPIKey      = errors.New("no API key found")
	ErrInvalidAPIKey = errors.New("invalid API key")
	ErrAPIError      = errors.New("API error")
)

// NewProvider creates a new LLM provider based on the type
func NewProvider(providerType ProviderType) (Provider, error) {
	switch providerType {
	case ProviderOpenAI:
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, ErrNoAPIKey
		}
		return NewOpenAIProvider(apiKey)
	case ProviderAnthropic:
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, ErrNoAPIKey
		}
		return NewAnthropicProvider(apiKey)
	default:
		return nil, errors.New("unknown provider type: " + string(providerType))
	}
}
