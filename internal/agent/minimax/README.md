# MiniMax Coding Plan Provider

This package provides support for MiniMax Coding Plan models in Crush.

## Models

The MiniMax provider includes two models:

- **MiniMax-M2**: 197K context window, optimized for code generation and reasoning
- **MiniMax-M2.1**: 205K context window, enhanced version with improved capabilities

Both models support:
- Code generation and reasoning
- Multi-turn dialogue
- Code debugging
- Multi-file context

## Usage

To enable the MiniMax provider, set the `MINIMAX` environment variable:

```bash
export MINIMAX=1
crush
```

You can also use the alternative environment variables:
- `MINIMAX_ENABLE=1`
- `MINIMAX_ENABLED=1`

## API Configuration

The provider uses the MiniMax Anthropic-compatible API endpoint by default:
```
https://api.minimax.io/anthropic
```

To use a custom endpoint, set the `MINIMAX_URL` environment variable:

```bash
export MINIMAX_URL=https://custom.minimax.io
export MINIMAX=1
crush
```

## Authentication

You'll need to configure your MiniMax API key in Crush's provider configuration. The API key should be set according to Crush's standard provider authentication mechanism.

## More Information

For more details about the MiniMax Coding Plan, visit:
- [MiniMax Coding Plan Quickstart](https://platform.minimax.io/docs/coding-plan/quickstart)
- [MiniMax API Documentation](https://platform.minimax.io/docs)
