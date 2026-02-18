package llmadvice

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

const (
	anthropicModel = "claude-3-5-haiku-latest"
)

// AnthropicProvider implements the Provider interface for Anthropic
type AnthropicProvider struct {
	llm   llms.Model
	model string
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey string) (*AnthropicProvider, error) {
	llm, err := anthropic.New(
		anthropic.WithToken(apiKey),
		anthropic.WithModel(anthropicModel),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Anthropic client: %w", err)
	}
	return &AnthropicProvider{
		llm:   llm,
		model: anthropicModel,
	}, nil
}

func (p *AnthropicProvider) Name() string {
	return string(ProviderAnthropic)
}

func (p *AnthropicProvider) Model() string {
	return p.model
}

func (p *AnthropicProvider) GenerateAdvice(ctx context.Context, prompt string) ([]string, error) {
	response, err := llms.GenerateFromSinglePrompt(ctx, p.llm, prompt,
		llms.WithTemperature(0.3),
		llms.WithMaxTokens(500),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAPIError, err)
	}

	return parseAdviceResponse(response), nil
}
