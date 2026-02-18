# git-explain

Shows contribution status across git repositories.

## Flow

main.go -> analyzer.AnalyzeRepo() -> RepoInfo -> render.RenderRepo() -> stdout

Optional: llmadvice.GetLLMAdvice() for LLM-powered suggestions.

## Packages

- internal/analyzer — produces RepoInfo from git repo
- internal/render — terminal formatting (compact/verbose/table/JSON)
- internal/llmadvice — LLM advice with caching

## LLM Advice

Enabled with --llm-advice. Requires OPENAI_API_KEY or ANTHROPIC_API_KEY.

Cache location: XDG_CACHE_HOME/git-this-bread/llm-advice/
Cache key: hash of repo state (branch, ahead/behind, dirty files, etc.)

## Required Git Config

Tool identifies user via:
- user.email — matches commit authors
- github.user — matches remote URLs
