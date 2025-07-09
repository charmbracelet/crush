![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/googleapis/go-genai)
[![Go Reference](https://pkg.go.dev/badge/google.golang.org/genai.svg)](https://pkg.go.dev/google.golang.org/genai)

## ✨ NEW ✨

### Google Gemini Multimodal Live support

Introducing support for the Gemini Multimodal Live feature. Here's an example Multimodal Live server showing realtime conversation and video streaming: [code](./examples/live/live_streaming_server.go)

# Google Gen AI Go SDK

The Google Gen AI Go SDK enables developers to use Google's state-of-the-art
generative AI models (like Gemini) to build AI-powered features and applications.
This SDK supports use cases like:
- Generate text from text-only input
- Generate text from text-and-images input (multimodal)
- ...

For example, with just a few lines of code, you can access Gemini's multimodal
capabilities to generate text from text-and-image input.

```go
parts := []*genai.Part{
  {Text: "What's this image about?"},
  {InlineData: &genai.Blob{Data: imageBytes, MIMEType: "image/jpeg"}},
}
result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash", []*genai.Content{{Parts: parts}}, nil)
```

## Installation and usage

Add the SDK to your module with `go get google.golang.org/genai`.

## Create Clients

### Imports
```go
import "google.golang.org/genai"
```

### Gemini API Client:
```go
client, err := genai.NewClient(ctx, &genai.ClientConfig{
	APIKey:   apiKey,
	Backend:  genai.BackendGeminiAPI,
})
```

### Vertex AI Client:
```go
client, err := genai.NewClient(ctx, &genai.ClientConfig{
	Project:  project,
	Location: location,
	Backend:  genai.BackendVertexAI,
})
```

## License

The contents of this repository are licensed under the
[Apache License, version 2.0](http://www.apache.org/licenses/LICENSE-2.0).
