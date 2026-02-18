package llmadvice

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	openAIModel = "gpt-4o-mini"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	llm   llms.Model
	model string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey string) (*OpenAIProvider, error) {
	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithModel(openAIModel),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
	}
	return &OpenAIProvider{
		llm:   llm,
		model: openAIModel,
	}, nil
}

func (p *OpenAIProvider) Name() string {
	return string(ProviderOpenAI)
}

func (p *OpenAIProvider) Model() string {
	return p.model
}

func (p *OpenAIProvider) GenerateAdvice(ctx context.Context, prompt string) ([]string, error) {
	response, err := llms.GenerateFromSinglePrompt(ctx, p.llm, prompt,
		llms.WithTemperature(0.3),
		llms.WithMaxTokens(500),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAPIError, err)
	}

	return parseAdviceResponse(response), nil
}
