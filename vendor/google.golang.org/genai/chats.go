// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Chats client.

package genai

import (
	"context"
	"io"
	"iter"
	"log"
)

// Chats provides util functions for creating a new chat session.
// You don't need to initiate this struct. Create a client instance via NewClient, and
// then access Chats through client.Models field.
type Chats struct {
	apiClient *apiClient
}

// Chat represents a single chat session (multi-turn conversation) with the model.
//
//		client, _ := genai.NewClient(ctx, &genai.ClientConfig{})
//		chat, _ := client.Chats.Create(ctx, "gemini-2.0-flash", nil, nil)
//	  result, err = chat.SendMessage(ctx, genai.Part{Text: "What is 1 + 2?"})
type Chat struct {
	Models
	apiClient *apiClient
	model     string
	config    *GenerateContentConfig
	// History of the chat.
	comprehensiveHistory []*Content
}

// Create initializes a new chat session.
func (c *Chats) Create(ctx context.Context, model string, config *GenerateContentConfig, history []*Content) (*Chat, error) {
	chat := &Chat{
		apiClient:            c.apiClient,
		model:                model,
		config:               config,
		comprehensiveHistory: history,
	}
	chat.Models.apiClient = c.apiClient
	return chat, nil
}

func (c *Chat) recordHistory(ctx context.Context, inputContent *Content, outputContents []*Content) {
	c.comprehensiveHistory = append(c.comprehensiveHistory, inputContent)

	for _, outputContent := range outputContents {
		c.comprehensiveHistory = append(c.comprehensiveHistory, copySanitizedModelContent(outputContent))
	}
}

// copySanitizedModelContent creates a (shallow) copy of modelContent with role set to
// model and all Parts copied verbatim.
func copySanitizedModelContent(modelContent *Content) *Content {
	newContent := &Content{Role: RoleModel}
	newContent.Parts = append(newContent.Parts, modelContent.Parts...)
	return newContent
}

// History returns the chat history. Curated (valid only) history is not supported yet.
func (c *Chat) History(curated bool) []*Content {
	if curated {
		log.Println("curated history is not supported yet")
		return nil
	}
	return c.comprehensiveHistory
}

// SendMessage sends the conversation history with the additional user's message and returns the model's response.
func (c *Chat) SendMessage(ctx context.Context, parts ...Part) (*GenerateContentResponse, error) {
	// Transform Parts to single Content
	p := make([]*Part, len(parts))
	for i, part := range parts {
		p[i] = &part
	}
	inputContent := &Content{Parts: p, Role: RoleUser}

	// Combine history with input content to send to model
	contents := append(c.comprehensiveHistory, inputContent)

	// Generate Content
	modelOutput, err := c.GenerateContent(ctx, c.model, contents, c.config)
	if err != nil {
		return nil, err
	}

	// Record history. By default, use the first candidate for history.
	var outputContents []*Content
	if len(modelOutput.Candidates) > 0 && modelOutput.Candidates[0].Content != nil {
		outputContents = append(outputContents, modelOutput.Candidates[0].Content)
	}
	c.recordHistory(ctx, inputContent, outputContents)

	return modelOutput, err
}

// SendMessageStream sends the conversation history with the additional user's message and returns the model's response.
func (c *Chat) SendMessageStream(ctx context.Context, parts ...Part) iter.Seq2[*GenerateContentResponse, error] {
	// Transform Parts to single Content
	p := make([]*Part, len(parts))
	for i, part := range parts {
		p[i] = &part
	}
	inputContent := &Content{Parts: p, Role: "user"}

	// Combine history with input content to send to model
	contents := append(c.comprehensiveHistory, inputContent)

	// Generate Content
	response := c.GenerateContentStream(ctx, c.model, contents, c.config)

	// Return a new iterator that will yield the responses and record history with merged response.
	return func(yield func(*GenerateContentResponse, error) bool) {
		var outputContents []*Content
		for chunk, err := range response {
			if err == io.EOF {
				break
			}
			if err != nil {
				yield(nil, err)
				return
			}
			if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
				outputContents = append(outputContents, chunk.Candidates[0].Content)
			}
			yield(chunk, nil)
		}
		// Record history. By default, use the first candidate for history.
		c.recordHistory(ctx, inputContent, outputContents)
	}
}
