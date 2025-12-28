package summarizer

// OpenAIModel represents an OpenAI model suitable for summarization.
type OpenAIModel struct {
	ID          string // API model ID
	Name        string // Display name
	Description string // Brief description
	Tier        string // "flagship", "standard", "fast", "economy", "reasoning", "legacy"
}

// OpenAIModels lists models suitable for text summarization.
// Excludes: image, audio, video, embedding, moderation, codex, transcription models.
// Updated: December 2025
var OpenAIModels = []OpenAIModel{
	// Flagship models (best quality)
	{ID: "gpt-5.2", Name: "GPT-5.2", Description: "Latest and most capable model", Tier: "flagship"},
	{ID: "gpt-5.2-pro", Name: "GPT-5.2 Pro", Description: "Smarter, more precise responses", Tier: "flagship"},
	{ID: "gpt-5.1", Name: "GPT-5.1", Description: "Excellent for complex tasks", Tier: "flagship"},
	{ID: "gpt-5-pro", Name: "GPT-5 Pro", Description: "Enhanced GPT-5 responses", Tier: "flagship"},
	{ID: "gpt-5", Name: "GPT-5", Description: "Previous flagship model", Tier: "flagship"},

	// Standard models (good balance)
	{ID: "gpt-4.1", Name: "GPT-4.1", Description: "Smartest non-reasoning model", Tier: "standard"},
	{ID: "gpt-4o", Name: "GPT-4o", Description: "Fast, intelligent, flexible", Tier: "standard"},
	{ID: "chatgpt-4o-latest", Name: "ChatGPT-4o", Description: "GPT-4o as used in ChatGPT", Tier: "standard"},

	// Fast models (speed optimized)
	{ID: "gpt-5-mini", Name: "GPT-5 Mini", Description: "Faster GPT-5 for defined tasks", Tier: "fast"},
	{ID: "gpt-4.1-mini", Name: "GPT-4.1 Mini", Description: "Faster version of GPT-4.1", Tier: "fast"},
	{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Description: "Fast, affordable for focused tasks", Tier: "fast"},

	// Economy models (cost optimized)
	{ID: "gpt-5-nano", Name: "GPT-5 Nano", Description: "Most cost-efficient GPT-5", Tier: "economy"},
	{ID: "gpt-4.1-nano", Name: "GPT-4.1 Nano", Description: "Most cost-efficient GPT-4.1", Tier: "economy"},

	// Reasoning models (complex analysis)
	{ID: "o3-pro", Name: "o3 Pro", Description: "Most compute for best responses", Tier: "reasoning"},
	{ID: "o3", Name: "o3", Description: "Reasoning for complex tasks", Tier: "reasoning"},
	{ID: "o3-mini", Name: "o3 Mini", Description: "Smaller alternative to o3", Tier: "reasoning"},
	{ID: "o4-mini", Name: "o4 Mini", Description: "Fast, cost-efficient reasoning", Tier: "reasoning"},
	{ID: "o1-pro", Name: "o1 Pro", Description: "Enhanced o1 responses", Tier: "reasoning"},
	{ID: "o1", Name: "o1", Description: "Previous o-series reasoning model", Tier: "reasoning"},

	// Legacy models (still available)
	{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Description: "Older high-intelligence model", Tier: "legacy"},
	{ID: "gpt-4", Name: "GPT-4", Description: "Original GPT-4", Tier: "legacy"},
	{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Description: "Cheap legacy model", Tier: "legacy"},
}

// DefaultOpenAIModel is the recommended model for summarization.
// gpt-5-nano offers the best cost-efficiency while maintaining good quality.
const DefaultOpenAIModel = "gpt-5-nano"

// GetOpenAIModelByID returns model info by ID, or nil if not found.
func GetOpenAIModelByID(id string) *OpenAIModel {
	for _, m := range OpenAIModels {
		if m.ID == id {
			return &m
		}
	}
	return nil
}

// GetOpenAIModelsByTier returns all models of a given tier.
func GetOpenAIModelsByTier(tier string) []OpenAIModel {
	var result []OpenAIModel
	for _, m := range OpenAIModels {
		if m.Tier == tier {
			result = append(result, m)
		}
	}
	return result
}
