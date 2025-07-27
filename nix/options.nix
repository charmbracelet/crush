{ lib }:

lib.mkOption {
  type = lib.types.submodule {
    options = {
      providers = lib.mkOption {
        type = lib.types.attrsOf (
          lib.types.submodule {
            options = {
              name = lib.mkOption {
                type = lib.types.str;
                description = "Provider display name";
              };
              type = lib.mkOption {
                type = lib.types.str;
                default = "openai";
                description = "Provider type (openai, anthropic, etc.)";
              };
              base_url = lib.mkOption {
                type = lib.types.str;
                default = "";
                description = "Provider API endpoint";
              };
              api_key = lib.mkOption {
                type = lib.types.str;
                default = "";
                description = "Provider API key";
              };
              system_prompt_prefix = lib.mkOption {
                type = lib.types.str;
                default = "";
                description = "prefix for the system prompt";
              };
              extra_headers = lib.mkOption {
                type = lib.types.attrsOf lib.types.str;
                default = { };
                description = "Extra HTTP headers for the provider";
              };
              models = lib.mkOption {
                type = lib.types.listOf (
                  lib.types.submodule {
                    options = {
                      id = lib.mkOption {
                        type = lib.types.str;
                        description = "Model ID";
                      };
                      model = lib.mkOption {
                        type = lib.types.str;
                        description = "Model display name";
                      };
                      cost_per_1m_in = lib.mkOption {
                        type = lib.types.int;
                        default = 0;
                        description = "Cost per 1M input tokens";
                      };
                      cost_per_1m_out = lib.mkOption {
                        type = lib.types.int;
                        default = 0;
                        description = "Cost per 1M output tokens";
                      };
                      cost_per_1m_in_cached = lib.mkOption {
                        type = lib.types.int;
                        default = 0;
                        description = "Cost per 1M cached input tokens";
                      };
                      cost_per_1m_out_cached = lib.mkOption {
                        type = lib.types.int;
                        default = 0;
                        description = "Cost per 1M cached output tokens";
                      };
                      context_window = lib.mkOption {
                        type = lib.types.int;
                        default = 128000;
                        description = "Model context window size";
                      };
                      default_max_tokens = lib.mkOption {
                        type = lib.types.int;
                        default = 8192;
                        description = "Default max tokens for responses";
                      };
                      can_reason = lib.mkOption {
                        type = lib.types.bool;
                        default = false;
                        description = "Whether the model can reason";
                      };
                      has_reasoning_efforts = lib.mkOption {
                        type = lib.types.bool;
                        default = false;
                        description = "Whether the model has reasoning efforts";
                      };
                      supports_attachments = lib.mkOption {
                        type = lib.types.bool;
                        default = false;
                        description = "Whether the model supports attachments";
                      };
                    };
                  }
                );
                default = [ ];
                description = "Provider models configuration";
              };
              disable = lib.mkOption {
                type = lib.types.bool;
                default = false;
                description = "Disable this provider";
              };
            };
          }
        );
        default = { };
        description = "Provider configurations";
      };
      lsp = lib.mkOption {
        type = lib.types.attrsOf (
          lib.types.submodule {
            options = {
              command = lib.mkOption {
                type = lib.types.str;
                description = "LSP command to execute";
              };
              args = lib.mkOption {
                type = lib.types.listOf lib.types.str;
                default = [ ];
                description = "Arguments to pass to the LSP command";
              };
              disabled = lib.mkOption {
                type = lib.types.bool;
                default = false;
                description = "Disable this LSP";
              };
            };
          }
        );
        default = { };
        description = "LSP configurations";
      };
      mcp = lib.mkOption {
        type = lib.types.attrsOf (
          lib.types.submodule {
            options = {
              type = lib.mkOption {
                type = lib.types.str;
                default = "http";
                description = "MCP type (http, stdio, sse)";
              };
              url = lib.mkOption {
                type = lib.types.str;
                default = "";
                description = "MCP URL for HTTP type";
              };
              command = lib.mkOption {
                type = lib.types.str;
                default = "";
                description = "MCP command for stdio/sse type";
              };
              args = lib.mkOption {
                type = lib.types.listOf lib.types.str;
                default = [ ];
                description = "Arguments for MCP command";
              };
              headers = lib.mkOption {
                type = lib.types.attrsOf lib.types.str;
                default = { };
                description = "HTTP headers for MCP";
              };
              disabled = lib.mkOption {
                type = lib.types.bool;
                default = false;
                description = "Disable this MCP";
              };
            };
          }
        );
        default = { };
        description = "MCP configurations";
      };
      options = lib.mkOption {
        type = lib.types.submodule {
          options = {
            context_paths = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "Additional context paths";
            };
            tui = lib.mkOption {
              type = lib.types.submodule {
                options = {
                  compact_mode = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Enable compact mode in the TUI";
                  };
                };
              };
              default = { };
              description = "TUI options";
            };
            debug = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Enable debug mode";
            };
            debug_lsp = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Enable LSP debugging";
            };
            disable_auto_summarize = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Disable automatic summarization";
            };
            data_directory = lib.mkOption {
              type = lib.types.str;
              default = ".crush";
              description = "Data directory relative to working directory";
            };
          };
        };
        default = { };
        description = "General options";
      };
      models = lib.mkOption {
        type = lib.types.attrsOf (
          lib.types.submodule {
            options = {
              model = lib.mkOption {
                type = lib.types.str;
                description = "Model ID as used by the provider API";
              };
              provider = lib.mkOption {
                type = lib.types.str;
                description = "Model provider ID";
              };
              reasoning_effort = lib.mkOption {
                type = lib.types.str;
                default = "";
                description = "Reasoning effort for OpenAI models";
              };
              max_tokens = lib.mkOption {
                type = lib.types.int;
                default = 0;
                description = "Override default max tokens";
              };
              think = lib.mkOption {
                type = lib.types.bool;
                default = false;
                description = "Enable thinking for Anthropic models";
              };
            };
          }
        );
        default = { };
        description = "Model configurations";
      };
    };
  };
  default = { };
  description = "Crush configuration options";
}
