import express from 'express';
import cors from 'cors';
import { createAnthropic } from '@ai-sdk/anthropic';
import { generateText, streamText } from 'ai';

const app = express();
app.use(cors());
app.use(express.json({ limit: '50mb' }));

const PORT = process.env.AI_SDK_BRIDGE_PORT || 8765;

// Health check endpoint
app.get('/health', (req, res) => {
  res.json({ status: 'ok', service: 'lash-ai-sdk-bridge' });
});

// Main completion endpoint that matches Anthropic API format
app.post('/v1/messages', async (req, res) => {
  try {
    const { 
      model, 
      messages, 
      max_tokens, 
      temperature = 0,
      stream = false,
      system,
      tools
    } = req.body;

    // Extract auth from headers
    const authHeader = req.headers.authorization;
    if (!authHeader || !authHeader.startsWith('Bearer ')) {
      return res.status(401).json({
        type: 'error',
        error: {
          type: 'authentication_error',
          message: 'Missing or invalid authorization header'
        }
      });
    }

    const accessToken = authHeader.substring(7);

    // Create Anthropic provider with custom fetch for OAuth - matching OpenCode exactly
    const anthropic = createAnthropic({
      apiKey: '', // Empty API key since we're using OAuth
      fetch: async (input, init) => {
        // Log the request for debugging
        console.log('AI SDK Request:', {
          url: input,
          method: init?.method,
          hasBody: !!init?.body
        });
        
        const headers = {
          ...init.headers,
          'authorization': `Bearer ${accessToken}`,
          'anthropic-beta': 'oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14'
        };
        // Critical: delete x-api-key AFTER setting other headers
        delete headers['x-api-key'];
        delete headers['X-Api-Key'];
        
        const response = await fetch(input, {
          ...init,
          headers
        });
        
        // Log response for debugging
        console.log('AI SDK Response:', {
          status: response.status,
          statusText: response.statusText
        });
        
        return response;
      }
    });

    // Convert messages to AI SDK format
    const aiMessages = messages.map(msg => {
      // Handle Anthropic format where content might be an array
      let content = msg.content;
      if (Array.isArray(content)) {
        // Extract text from content blocks
        content = content
          .filter(block => block.type === 'text' || block.text)
          .map(block => block.text || block.content)
          .join('');
      }
      
      if (msg.role === 'user') {
        return { role: 'user', content };
      } else if (msg.role === 'assistant') {
        return { role: 'assistant', content };
      } else if (msg.role === 'system') {
        return { role: 'system', content };
      }
      return msg;
    });

    // Add system message if provided (can be string or array)
    if (system) {
      let systemContent = system;
      if (Array.isArray(system)) {
        systemContent = system
          .filter(block => block.type === 'text' || block.text)
          .map(block => block.text || block.content)
          .join('');
      }
      // Only add if not already present as first message
      if (!aiMessages[0] || aiMessages[0].role !== 'system') {
        aiMessages.unshift({ role: 'system', content: systemContent });
      }
    }

    if (stream) {
      // Handle streaming response
      res.setHeader('Content-Type', 'text/event-stream');
      res.setHeader('Cache-Control', 'no-cache');
      res.setHeader('Connection', 'keep-alive');

      const result = await streamText({
        model: anthropic(model),
        messages: aiMessages,
        maxTokens: max_tokens,
        temperature,
        tools: tools ? convertTools(tools) : undefined
      });

      for await (const chunk of result.textStream) {
        const event = {
          type: 'content_block_delta',
          delta: { type: 'text_delta', text: chunk }
        };
        res.write(`data: ${JSON.stringify(event)}\n\n`);
      }

      res.write('data: {"type":"message_stop"}\n\n');
      res.end();
    } else {
      // Handle non-streaming response
      const result = await generateText({
        model: anthropic(model),
        messages: aiMessages,
        maxTokens: max_tokens,
        temperature,
        tools: tools ? convertTools(tools) : undefined
      });

      // Convert AI SDK response to Anthropic API format
      const response = {
        id: `msg_${Date.now()}`,
        type: 'message',
        role: 'assistant',
        content: [
          {
            type: 'text',
            text: result.text
          }
        ],
        model,
        stop_reason: result.finishReason || 'end_turn',
        stop_sequence: null,
        usage: {
          input_tokens: result.usage?.promptTokens || 0,
          output_tokens: result.usage?.completionTokens || 0
        }
      };

      res.json(response);
    }
  } catch (error) {
    console.error('Error processing request:', error);
    
    // Check if it's a Claude Code restriction error
    if (error.message && error.message.includes('authorized for use with Claude Code')) {
      return res.status(400).json({
        type: 'error',
        error: {
          type: 'invalid_request_error',
          message: error.message
        }
      });
    }

    res.status(500).json({
      type: 'error',
      error: {
        type: 'internal_error',
        message: error.message || 'Internal server error'
      }
    });
  }
});

function convertTools(anthropicTools) {
  // Convert Anthropic tool format to AI SDK format
  if (!anthropicTools || anthropicTools.length === 0) return undefined;
  
  return anthropicTools.reduce((acc, tool) => {
    // Ensure input_schema has a type field
    const schema = tool.input_schema || {};
    if (!schema.type) {
      schema.type = 'object';
    }
    if (!schema.properties) {
      schema.properties = {};
    }
    
    acc[tool.name] = {
      description: tool.description,
      parameters: schema
    };
    return acc;
  }, {});
}

app.listen(PORT, () => {
  console.log(`AI SDK Bridge listening on port ${PORT}`);
  console.log(`Health check: http://localhost:${PORT}/health`);
});