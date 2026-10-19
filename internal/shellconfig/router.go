package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
)

// handleRouter implements the `router` builtin.
//
// Usage:
//
//	router add <name> --classifier <provider>/<id> [--default ROUTE]
//	    [--instructions TEXT] [--timeout N]
//	router route <name> <route> <provider>/<id> --description TEXT
//	router remove <name> [<route>]   (alias: rm)
//	router use <name> | router use --none
//
// "add" declares (or updates) a router. "route" adds or replaces a route on
// an existing router. "remove" deletes a router, or one of its routes.
// "use" activates a router in place of the large model.
func handleRouter(ctx context.Context, args []string, _ io.Reader, _, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: router add|route|remove|use ...")
	}
	switch args[1] {
	case "add":
		return routerAdd(b, args, stderr)
	case "route":
		return routerRoute(b, args, stderr)
	case "remove", "rm":
		return routerRemove(b, args, stderr)
	case "use":
		return routerUse(b, args, stderr)
	default:
		return usage(stderr, fmt.Sprintf("router: unknown subcommand %q (expected add, route, remove, or use)", args[1]))
	}
}

var routerAddFlags = []flagSpec{
	{name: "--classifier", jsonKey: "classifier", kind: flagString, op: opSet},
	{name: "--default", jsonKey: "default", kind: flagString, op: opSet},
	{name: "--instructions", jsonKey: "instructions", kind: flagString, op: opSet},
	{name: "--timeout", jsonKey: "timeout", kind: flagInt, op: opSet},
}

func routerAdd(b *ConfigBuilder, args []string, stderr io.Writer) error {
	const help = "usage: router add <name> --classifier <provider>/<id> [--default ROUTE] [--instructions TEXT] [--timeout N]"
	if len(args) < 3 {
		return usage(stderr, help)
	}
	name := args[2]
	r := childMap(b.section("routers"), name)
	parsed := map[string]any{}
	if err := applyFlags(routerAddFlags, args, 3, parsed, "router add", stderr); err != nil {
		return err
	}
	if c, ok := parsed["classifier"].(string); ok {
		provider, id, ok := splitProviderModel(c)
		if !ok {
			return usage(stderr, fmt.Sprintf("router add: --classifier expects <provider>/<id>, got %q", c))
		}
		r["classifier"] = map[string]any{"provider": provider, "model": id}
		delete(parsed, "classifier")
	}
	if _, ok := r["classifier"]; !ok {
		return usage(stderr, help)
	}
	maps.Copy(r, parsed)
	slog.Info("Router added in shell config", "router", name)
	return nil
}

var routerRouteFlags = []flagSpec{
	{name: "--description", jsonKey: "description", kind: flagString, op: opSet},
}

func routerRoute(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 5 {
		return usage(stderr, "usage: router route <name> <route> <provider>/<id> --description TEXT")
	}
	name, routeName := args[2], args[3]
	r, ok := b.section("routers")[name].(map[string]any)
	if !ok {
		return usage(stderr, fmt.Sprintf("router route: router %q does not exist (declare it with `router add %s` first)", name, name))
	}
	provider, id, ok := splitProviderModel(args[4])
	if !ok {
		return usage(stderr, fmt.Sprintf("router route: expected <provider>/<id>, got %q", args[4]))
	}
	route := map[string]any{
		"name":  routeName,
		"model": map[string]any{"provider": provider, "model": id},
	}
	if err := applyFlags(routerRouteFlags, args, 5, route, "router route", stderr); err != nil {
		return err
	}
	if _, ok := route["description"]; !ok {
		return usage(stderr, "router route: --description is required")
	}

	routes, _ := r["routes"].([]any)
	kept := make([]any, 0, len(routes)+1)
	replaced := false
	for _, item := range routes {
		if m, ok := item.(map[string]any); ok && m["name"] == routeName {
			kept = append(kept, route)
			replaced = true
			continue
		}
		kept = append(kept, item)
	}
	if !replaced {
		kept = append(kept, route)
	}
	r["routes"] = kept
	slog.Info("Router route set in shell config", "router", name, "route", routeName)
	return nil
}

func routerRemove(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: router remove <name> [<route>]")
	}
	name := args[2]
	routers := b.section("routers")
	if len(args) == 3 {
		delete(routers, name)
		if b.root["active_router"] == name {
			b.root["active_router"] = ""
		}
		slog.Info("Router removed in shell config", "router", name)
		return nil
	}
	r, ok := routers[name].(map[string]any)
	if !ok {
		return nil
	}
	routes, _ := r["routes"].([]any)
	kept := make([]any, 0, len(routes))
	for _, item := range routes {
		if m, ok := item.(map[string]any); ok && m["name"] == args[3] {
			continue
		}
		kept = append(kept, item)
	}
	r["routes"] = kept
	slog.Info("Router route removed in shell config", "router", name, "route", args[3])
	return nil
}

func routerUse(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) != 3 {
		return usage(stderr, "usage: router use <name> | router use --none")
	}
	if args[2] == "--none" {
		b.root["active_router"] = ""
		return nil
	}
	b.root["active_router"] = args[2]
	slog.Info("Router activated in shell config", "router", args[2])
	return nil
}
