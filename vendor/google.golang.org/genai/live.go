// Copyright 2024 Google LLC
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

package genai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

// Preview. Live serves as the entry point for establishing real-time WebSocket
// connections to the API. It manages the initial handshake and setup process.
//
// It is initiated when creating a client via [NewClient]. You don't need to
// create a new Live object directly. Access it through the `Live` field of a
// `Client` instance.
//
//	client, _ := genai.NewClient(ctx, &genai.ClientConfig{})
//	session, _ := client.Live.Connect(ctx, model, &genai.LiveConnectConfig{}).
type Live struct {
	apiClient *apiClient
}

// Preview. Session represents an active, real-time WebSocket connection to the
// Generative AI API. It provides methods for sending client messages and
// receiving server messages over the established connection.
type Session struct {
	conn      *websocket.Conn
	apiClient *apiClient
}

// Preview. Connect establishes a WebSocket connection to the specified
// model with the given configuration. It sends the initial
// setup message and returns a [Session] object representing the connection.
func (r *Live) Connect(context context.Context, model string, config *LiveConnectConfig) (*Session, error) {
	httpOptions := r.apiClient.clientConfig.HTTPOptions
	if httpOptions.APIVersion == "" {
		return nil, fmt.Errorf("live module requires APIVersion to be set. You can set APIVersion to v1beta1 for BackendVertexAI or v1apha for BackendGeminiAPI")
	}
	baseURL, err := url.Parse(httpOptions.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}
	scheme := baseURL.Scheme
	// Avoid overwrite schema if websocket scheme is already specified.
	if scheme != "wss" && scheme != "ws" {
		scheme = "wss"
	}

	var u url.URL
	// TODO(b/406076143): Support function level httpOptions.
	var header http.Header = mergeHeaders(&httpOptions, nil)
	if r.apiClient.clientConfig.Backend == BackendVertexAI {
		token, err := r.apiClient.clientConfig.Credentials.Token(context)
		if err != nil {
			return nil, fmt.Errorf("failed to get token: %w", err)
		}
		header.Set("Authorization", fmt.Sprintf("Bearer %s", token.Value))
		u = url.URL{
			Scheme: scheme,
			Host:   baseURL.Host,
			Path:   fmt.Sprintf("%s/ws/google.cloud.aiplatform.%s.LlmBidiService/BidiGenerateContent", baseURL.Path, httpOptions.APIVersion),
		}
	} else {
		u = url.URL{
			Scheme:   scheme,
			Host:     baseURL.Host,
			Path:     fmt.Sprintf("%s/ws/google.ai.generativelanguage.%s.GenerativeService.BidiGenerateContent", baseURL.Path, httpOptions.APIVersion),
			RawQuery: fmt.Sprintf("key=%s", r.apiClient.clientConfig.APIKey),
		}
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		return nil, fmt.Errorf("Connect to %s failed: %w", u.String(), err)
	}
	s := &Session{
		conn:      conn,
		apiClient: r.apiClient,
	}
	modelFullName, err := tModelFullName(r.apiClient, model)
	if err != nil {
		return nil, err
	}
	kwargs := map[string]any{"model": modelFullName, "config": config}
	parameterMap := make(map[string]any)
	err = deepMarshal(kwargs, &parameterMap)
	if err != nil {
		return nil, err
	}

	var toConverter func(*apiClient, map[string]any, map[string]any) (map[string]any, error)
	if r.apiClient.clientConfig.Backend == BackendVertexAI {
		toConverter = liveConnectParametersToVertex
	} else {
		toConverter = liveConnectParametersToMldev
	}
	body, err := toConverter(r.apiClient, parameterMap, nil)
	if err != nil {
		return nil, err
	}
	delete(body, "config")

	clientBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal LiveClientSetup failed: %w", err)
	}
	err = s.conn.WriteMessage(websocket.TextMessage, clientBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write LiveClientSetup: %w", err)
	}
	return s, nil
}

// Preview. LiveClientContentInput is the input for [SendClientContent].
type LiveClientContentInput = LiveSendClientContentParameters

// Preview. SendClientContent transmits non-realtime, turn-based content to the model
// over the established WebSocket connection.
//
// There are two primary ways to send messages in a live session:
// [SendClientContent] and [SendRealtimeInput].
//
// Messages sent via [SendClientContent] are added to the model's context strictly
// **in the order they are sent**. A conversation using [SendClientContent] is
// similar to using the [Chat.SendMessageStream] method, but the conversation
// history state is managed by the API server.
//
// Due to this ordering guarantee, the model might not respond as quickly to
// [SendClientContent] messages compared to SendRealtimeInput messages. This latency
// difference is most noticeable when sending content that requires significant
// preprocessing, such as images.
//
// [SendClientContent] accepts a LiveClientContentInput which contains a list of
// [*Content] objects, offering more flexibility than the [*Blob] used by
// SendRealtimeInput.
//
// Key use cases for [SendClientContent] over SendRealtimeInput include:
//   - Pre-populating the conversation context (including sending content types
//     not supported by realtime messages) before starting a realtime interaction.
//   - Conducting a non-realtime conversation, similar to client.Chats.SendMessage,
//     using the live API infrastructure.
//
// Caution: Interleaving [SendClientContent] and SendRealtimeInput within the
// same conversation is not recommended and may lead to unexpected behavior.
//
// The input parameter of type [LiveClientContentInput] contains:
//   - Turns: A slice of [*Content] objects representing the message(s) to send.
//   - TurnComplete: If true (the default), the model will reply immediately.
//     If false, the model waits for subsequent SendClientContent calls until
//     one is sent with TurnComplete set to true.
func (s *Session) SendClientContent(input LiveClientContentInput) error {
	return s.send(input.toLiveClientMessage())
}

// Preview. LiveRealtimeInput is the input for [SendRealtimeInput].
type LiveRealtimeInput = LiveSendRealtimeInputParameters

// Preview. SendRealtimeInput transmits realtime audio chunks and video frames (images)
// to the model over the established WebSocket connection.
//
// Use SendRealtimeInput for streaming audio and video data. The API automatically
// responds to audio based on voice activity detection (VAD).
//
// SendRealtimeInput is optimized for responsiveness, potentially at the expense
// of deterministic ordering. Audio and video tokens are added to the model's
// context as they become available, allowing for faster interaction.
//
// It accepts a [LiveRealtimeInput] parameter containing the media data.
// Only one argument (e.g., Media, Audio, Video, Text) should be provided per call.
func (s *Session) SendRealtimeInput(input LiveRealtimeInput) error {
	parameterMap := make(map[string]any)
	err := deepMarshal(input, &parameterMap)
	if err != nil {
		return err
	}

	var toConverter func(*apiClient, map[string]any, map[string]any) (map[string]any, error)
	if s.apiClient.clientConfig.Backend == BackendVertexAI {
		toConverter = liveSendRealtimeInputParametersToVertex
	} else {
		toConverter = liveSendRealtimeInputParametersToMldev
	}
	body, err := toConverter(s.apiClient, parameterMap, nil)
	if err != nil {
		return err
	}

	data, err := json.Marshal(map[string]any{"realtimeInput": body})
	if err != nil {
		return fmt.Errorf("marshal client message error: %w", err)
	}
	return s.conn.WriteMessage(websocket.TextMessage, []byte(data))
}

// Preview. LiveToolResponseInput is the input for [SendToolResponse].
type LiveToolResponseInput = LiveSendToolResponseParameters

// Preview. SendToolResponse transmits a [LiveClientToolResponse] over the established WebSocket connection.
//
// Use SendToolResponse to reply to [LiveServerToolCall] messages received from the server.
//
// To define the available tools for the session, set the [LiveConnectConfig.Tools]
// field when establishing the connection via [Live.Connect].
func (s *Session) SendToolResponse(input LiveToolResponseInput) error {
	return s.send(input.toLiveClientMessage())
}

// Send transmits a LiveClientMessage over the established connection.
// It returns an error if sending the message fails.
func (s *Session) send(input *LiveClientMessage) error {
	if input.Setup != nil {
		return fmt.Errorf("message SetUp is not supported in Send(). Use Connect() instead")
	}

	parameterMap := make(map[string]any)
	err := deepMarshal(input, &parameterMap)
	if err != nil {
		return err
	}

	var toConverter func(*apiClient, map[string]any, map[string]any) (map[string]any, error)
	if s.apiClient.clientConfig.Backend == BackendVertexAI {
		toConverter = liveClientMessageToVertex
	} else {
		toConverter = liveClientMessageToMldev
	}
	body, err := toConverter(s.apiClient, parameterMap, nil)
	if err != nil {
		return err
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal client message error: %w", err)
	}
	return s.conn.WriteMessage(websocket.TextMessage, []byte(data))
}

// Preview. Receive reads a LiveServerMessage from the connection.
//
// This method blocks until a message is received from the server.
// The returned message represents a part of or a complete model turn.
// If the received message is a [LiveServerToolCall], the user must call
// [SendToolResponse] to provide the function execution result and continue the turn.
func (s *Session) Receive() (*LiveServerMessage, error) {
	messageType, msgBytes, err := s.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	responseMap := make(map[string]any)
	err = json.Unmarshal(msgBytes, &responseMap)
	if err != nil {
		return nil, fmt.Errorf("invalid message format. Error %w. messageType: %d, message: %s", err, messageType, msgBytes)
	}
	if responseMap["error"] != nil {
		return nil, fmt.Errorf("received error in response: %v", string(msgBytes))
	}

	var fromConverter func(*apiClient, map[string]any, map[string]any) (map[string]any, error)
	if s.apiClient.clientConfig.Backend == BackendVertexAI {
		fromConverter = liveServerMessageFromVertex
	} else {
		fromConverter = liveServerMessageFromMldev
	}
	responseMap, err = fromConverter(s.apiClient, responseMap, nil)
	if err != nil {
		return nil, err
	}

	var message = new(LiveServerMessage)
	err = mapToStruct(responseMap, message)
	if err != nil {
		return nil, err
	}
	return message, err
}

// Preview. Close terminates the connection.
func (s *Session) Close() error {
	if s != nil && s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
