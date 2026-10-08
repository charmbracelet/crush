package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// handleProvider implements the `provider` builtin.
//
// Usage:
//
//	provider add <id> [--name NAME] [--type TYPE] [--api-key KEY]
//	    [--base-url URL] [--disable true|false] [--flat-rate true|false]
//	    [--discover-models true|false] [--system-prompt-prefix TEXT]
//	    [--extra-header KEY VALUE] [--extra-body JSON]
//	    [--provider-options JSON]
//	    [--oauth-issuer URL] [--oauth-client-id ID] [--oauth-scope SCOPE]
//	    [--oauth-flow auto|browser|device] ...
//	provider remove <id>   (alias: rm)
//
// "add" defines or updates a provider; repeated calls with the same <id>
// update the same entry. "remove" removes a provider and all its children.
//
// A provider authenticates with --api-key unless an --oauth-* flag declares
// an OAuth flow, which is what lets `crush login <id>` sign in to a provider
// Crush does not ship. A --usage-* flag declares where the provider reports
// the quota left on a plan, which `crush usage` reads. A --gateway-* flag
// declares the jq programs that rewrite the provider's traffic, which is how
// a provider whose wire format differs from what its SDK speaks is added
// without code. A --catalog-* flag declares the endpoint and jq program that
// read the provider's own model listing, so its models, context windows, and
// capability lists arrive from the wire rather than being copied into config.
func handleProvider(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: provider add <id> [flags] | provider remove <id>")
	}

	switch args[1] {
	case "add":
		return providerAdd(b, args, stderr)
	case "remove", "rm":
		return providerRemove(b, args, stderr)
	default:
		return usage(stderr, fmt.Sprintf("provider: unknown subcommand %q (expected add or remove)", args[1]))
	}
}

// providerAddFlags is the declarative flag surface for `provider add`.
var providerAddFlags = []flagSpec{
	{name: "--name", jsonKey: "name", kind: flagString, op: opSet},
	{name: "--type", jsonKey: "type", kind: flagString, op: opSet},
	{name: "--api-key", jsonKey: "api_key", kind: flagString, op: opSet},
	{name: "--base-url", jsonKey: "base_url", kind: flagString, op: opSet},
	{name: "--disable", jsonKey: "disable", kind: flagBool, op: opSet},
	{name: "--flat-rate", jsonKey: "flat_rate", kind: flagBool, op: opSet},
	{name: "--discover-models", jsonKey: "discover_models", kind: flagBool, op: opSet},
	{name: "--system-prompt-prefix", jsonKey: "system_prompt_prefix", kind: flagString, op: opSet},
	{name: "--extra-header", child: "extra_headers", kind: flagKeyValue, op: opSetChild},
	{name: "--extra-body", child: "extra_body", kind: flagJSONObject, op: opMergeChild},
	{name: "--provider-options", child: "provider_options", kind: flagJSONObject, op: opMergeChild},
	// OAuth declarations. Any of these marks the provider as signing in
	// rather than taking a key, so `crush login <id>` can authenticate it.
	{
		name: "--oauth-kind", child: "auth", jsonKey: "kind", kind: flagString, op: opSetChildValue,
		validate: oneOf("--oauth-kind", "oauth", "api_key"),
	},
	{
		name: "--oauth-flow", child: "auth", jsonKey: "flow", kind: flagString, op: opSetChildValue,
		validate: oneOf("--oauth-flow", "auto", "browser", "device"),
	},
	{name: "--oauth-issuer", child: "auth", jsonKey: "issuer", kind: flagString, op: opSetChildValue},
	{name: "--oauth-client-id", child: "auth", jsonKey: "client_id", kind: flagString, op: opSetChildValue},
	{name: "--oauth-client-secret", child: "auth", jsonKey: "client_secret", kind: flagString, op: opSetChildValue},
	{name: "--oauth-scope", child: "auth", jsonKey: "scopes", kind: flagString, op: opAppendChild},
	{name: "--oauth-auth-url", child: "auth", jsonKey: "authorize_url", kind: flagString, op: opSetChildValue},
	{name: "--oauth-token-url", child: "auth", jsonKey: "token_url", kind: flagString, op: opSetChildValue},
	{name: "--oauth-device-url", child: "auth", jsonKey: "device_auth_url", kind: flagString, op: opSetChildValue},
	{name: "--oauth-redirect-uri", child: "auth", jsonKey: "redirect_uri", kind: flagString, op: opSetChildValue},
	{name: "--oauth-callback-port", child: "auth", jsonKey: "callback_port", kind: flagInt, op: opSetChildValue},
	{name: "--oauth-param", child: "auth.extra_params", kind: flagKeyValue, op: opSetChild},
	{name: "--oauth-secret-basic", child: "auth", jsonKey: "client_secret_basic", kind: flagBool, op: opSetChildValue},
	{name: "--oauth-token-header", child: "auth.token_headers", kind: flagKeyValue, op: opSetChild},
	{
		name: "--oauth-token-encoding", child: "auth", jsonKey: "token_encoding", kind: flagString, op: opSetChildValue,
		validate: oneOf("--oauth-token-encoding", "form", "json"),
	},
	// Quota reporting. These declare where the provider reports the allowance
	// left on a subscription plan, which `crush usage` then reads.
	{name: "--usage-url", child: "usage", jsonKey: "url", kind: flagString, op: opSetChildValue},
	{name: "--usage-method", child: "usage", jsonKey: "method", kind: flagString, op: opSetChildValue},
	{name: "--usage-body", child: "usage", jsonKey: "body", kind: flagString, op: opSetChildValue},
	{name: "--usage-groups", child: "usage", jsonKey: "groups", kind: flagString, op: opSetChildValue},
	{name: "--usage-group-label", child: "usage", jsonKey: "group_label", kind: flagString, op: opSetChildValue},
	{name: "--usage-meters", child: "usage", jsonKey: "meters", kind: flagString, op: opSetChildValue},
	{name: "--usage-label", child: "usage", jsonKey: "label", kind: flagString, op: opSetChildValue},
	{name: "--usage-remaining", child: "usage", jsonKey: "remaining", kind: flagString, op: opSetChildValue},
	{name: "--usage-spent-percent", child: "usage", jsonKey: "spent_percent", kind: flagString, op: opSetChildValue},
	{name: "--usage-reset", child: "usage", jsonKey: "reset", kind: flagString, op: opSetChildValue},
	{name: "--usage-title", child: "usage", jsonKey: "title", kind: flagString, op: opSetChildValue},
	{name: "--usage-window", child: "usage", jsonKey: "window", kind: flagString, op: opSetChildValue},
	{name: "--usage-model-group", child: "usage.model_groups", kind: flagKeyValue, op: opSetChild},
	// Gateway adapter: jq programs that rewrite this provider's traffic, so a
	// provider whose wire format differs from its SDK's is configuration
	// rather than code.
	{name: "--gateway-request", child: "gateway", jsonKey: "request", kind: flagString, op: opSetChildValue},
	{name: "--gateway-response", child: "gateway", jsonKey: "response", kind: flagString, op: opSetChildValue},
	{name: "--gateway-http1", child: "gateway", jsonKey: "http1", kind: flagBool, op: opSetChildValue},
	// Model catalog: the endpoint and the jq program that read the provider's
	// own listing, so names, context windows, and capability lists come from
	// the provider instead of a list a plugin author keeps updating.
	{name: "--catalog-url", child: "catalog", jsonKey: "url", kind: flagString, op: opSetChildValue},
	{name: "--catalog-method", child: "catalog", jsonKey: "method", kind: flagString, op: opSetChildValue},
	{name: "--catalog-body", child: "catalog", jsonKey: "body", kind: flagString, op: opSetChildValue},
	{name: "--catalog-program", child: "catalog", jsonKey: "program", kind: flagString, op: opSetChildValue},
	{name: "--catalog-header", child: "catalog.headers", kind: flagKeyValue, op: opSetChild},
}

func providerAdd(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: provider add <id> [--name NAME] [--type TYPE] [--api-key KEY] [--base-url URL] [--disable true|false] [--flat-rate true|false] [--discover-models true|false] [--system-prompt-prefix TEXT] [--extra-header KEY VALUE] [--extra-body JSON] [--provider-options JSON] [--oauth-issuer URL] [--oauth-client-id ID] [--oauth-scope SCOPE] [--oauth-flow auto|browser|device] [--oauth-auth-url URL] [--oauth-token-url URL] [--oauth-device-url URL] [--oauth-redirect-uri URI] [--oauth-callback-port N] [--oauth-param KEY VALUE]")
	}
	id := args[2]
	slog.Info("Provider defined in shell config", "provider", id)
	p := childMap(b.section("providers"), id)

	if err := applyFlags(providerAddFlags, args, 3, p, "provider add", stderr); err != nil {
		return err
	}

	slog.Debug("Provider recorded", "provider", id)
	return nil
}

func providerRemove(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: provider remove <id>")
	}
	id := args[2]
	delete(b.section("providers"), id)
	slog.Info("Provider removed in shell config", "provider", id)
	return nil
}
