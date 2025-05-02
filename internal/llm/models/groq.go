package models

const (
	ProviderGROQ ModelProvider = "groq"

	// GROQ
	QWENQwq ModelID = "qwen-qwq"

	// GROQ preview models
	Llama4Scout               ModelID = "meta-llama/llama-4-scout-17b-16e-instruct"
	Llama4Maverick            ModelID = "meta-llama/llama-4-maverick-17b-128e-instruct"
	Llama3_3_70BVersatile     ModelID = "llama-3.3-70b-versatile"
	DeepseekR1DistillLlama70b ModelID = "deepseek-r1-distill-llama-70b"
)

var GroqModels = map[ModelID]Model{
	//
	// GROQ
	QWENQwq: {
		ID:                 QWENQwq,
		Name:               "Qwen Qwq",
		Provider:           ProviderGROQ,
		APIModel:           "qwen-qwq-32b",
		CostPer1MIn:        0.29,
		CostPer1MInCached:  0.275,
		CostPer1MOutCached: 0.0,
		CostPer1MOut:       0.39,
		ContextWindow:      128_000,
		DefaultMaxTokens:   50000,
		// for some reason, the groq api doesn't like the reasoningEffort parameter
		CanReason:           false,
		SupportsAttachments: false,
	},

	Llama4Scout: {
		ID:                  Llama4Scout,
		Name:                "Llama4Scout",
		Provider:            ProviderGROQ,
		APIModel:            "meta-llama/llama-4-scout-17b-16e-instruct",
		CostPer1MIn:         0.11,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.34,
		ContextWindow:       128_000, // 10M when?
		SupportsAttachments: true,
	},

	Llama4Maverick: {
		ID:                  Llama4Maverick,
		Name:                "Llama4Maverick",
		Provider:            ProviderGROQ,
		APIModel:            "meta-llama/llama-4-maverick-17b-128e-instruct",
		CostPer1MIn:         0.20,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.20,
		ContextWindow:       128_000,
		SupportsAttachments: true,
	},

	Llama3_3_70BVersatile: {
		ID:                  Llama3_3_70BVersatile,
		Name:                "Llama3_3_70BVersatile",
		Provider:            ProviderGROQ,
		APIModel:            "llama-3.3-70b-versatile",
		CostPer1MIn:         0.59,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.79,
		ContextWindow:       128_000,
		SupportsAttachments: false,
	},

	DeepseekR1DistillLlama70b: {
		ID:                  DeepseekR1DistillLlama70b,
		Name:                "DeepseekR1DistillLlama70b",
		Provider:            ProviderGROQ,
		APIModel:            "deepseek-r1-distill-llama-70b",
		CostPer1MIn:         0.75,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.99,
		ContextWindow:       128_000,
		CanReason:           true,
		SupportsAttachments: false,
	},
}
