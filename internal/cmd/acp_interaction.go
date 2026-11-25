package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	crushacp "github.com/charmbracelet/crush/internal/acp"
	"github.com/charmbracelet/x/ansi"
	acp "github.com/coder/acp-go-sdk"
)

type lineInput struct {
	mu     sync.Mutex
	reader *bufio.Scanner
}

func newLineInput(r io.Reader) *lineInput {
	return &lineInput{reader: bufio.NewScanner(r)}
}

func (in *lineInput) readLine(prompt string) (string, bool) {
	in.mu.Lock()
	defer in.mu.Unlock()
	fmt.Fprint(os.Stderr, prompt)
	if !in.reader.Scan() {
		return "", false
	}
	return strings.TrimSpace(in.reader.Text()), true
}

func safeTerminalText(text string) string {
	text = ansi.Strip(text)
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f || unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, text)
}

func newPermissionHandler(input *lineInput) crushacp.PermissionHandler {
	return func(ctx context.Context, req acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
		title, _ := req.ToolCall.Title.Value()
		fmt.Fprintf(os.Stderr, "\n[permission] %s\n", safeTerminalText(title))
		if len(req.Options) == 0 {
			return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
		}
		for i, option := range req.Options {
			fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, safeTerminalText(option.Name))
		}
		line, ok := input.readLine(fmt.Sprintf("Select option [1-%d], or [c]ancel: ", len(req.Options)))
		if !ok || strings.EqualFold(line, "c") || strings.EqualFold(line, "cancel") {
			return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
		}
		index, err := strconv.Atoi(line)
		if err != nil || index < 1 || index > len(req.Options) {
			return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
		}
		return acp.RequestPermissionResponse{
			Outcome: acp.NewRequestPermissionOutcomeSelected(req.Options[index-1].OptionId),
		}, nil
	}
}

func newElicitationHandler(input *lineInput) crushacp.ElicitationHandler {
	return func(ctx context.Context, req acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
		switch {
		case req.Form != nil:
			return elicitationForm(input, req.Form)
		case req.Url != nil:
			return elicitationURL(ctx, input, req.Url)
		case req.Other != nil:
			return acp.CreateElicitationResponse{}, acp.NewInvalidParams(map[string]any{"mode": req.Other.Mode})
		default:
			return acp.CreateElicitationResponse{}, acp.NewInvalidParams(map[string]any{"mode": ""})
		}
	}
}

func elicitationForm(input *lineInput, req *acp.CreateElicitationForm) (acp.CreateElicitationResponse, error) {
	fmt.Fprintf(os.Stderr, "\n[elicitation] %s\n", safeTerminalText(req.Message))
	line, ok := input.readLine("Accept? [y]es / [d]ecline / [c]ancel: ")
	if !ok {
		return cancelElicitation(), nil
	}
	switch strings.ToLower(line) {
	case "d", "decline":
		return declineElicitation(), nil
	case "c", "cancel":
		return cancelElicitation(), nil
	case "", "y", "yes":
	default:
		return cancelElicitation(), nil
	}

	names := make([]string, 0, len(req.RequestedSchema.Properties))
	for name := range req.RequestedSchema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	content := make(map[string]acp.ElicitationContentValue, len(names))
	for _, name := range names {
		property := req.RequestedSchema.Properties[name]
		label := propertyLabel(name, property)
		line, ok := input.readLine(label + ": ")
		if !ok {
			return cancelElicitation(), nil
		}
		if line == "" {
			value, found, err := propertyDefault(property)
			if err != nil {
				return declineElicitation(), nil
			}
			if found {
				content[name] = value
				continue
			}
		}
		value, err := parseElicitationValue(property, line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid value for %s: %v\n", name, err)
			return declineElicitation(), nil
		}
		content[name] = value
	}
	return acp.CreateElicitationResponse{
		Accept: &acp.CreateElicitationAccept{Action: "accept", Content: content},
	}, nil
}

func elicitationURL(ctx context.Context, input *lineInput, req *acp.CreateElicitationUrl) (acp.CreateElicitationResponse, error) {
	parsed, err := url.Parse(req.Url)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return acp.CreateElicitationResponse{}, acp.NewInvalidParams(map[string]any{"url": req.Url})
	}
	fmt.Fprintf(os.Stderr, "\n[elicitation] %s\n", safeTerminalText(req.Message))
	fmt.Fprintf(os.Stderr, "URL: %s\n", safeTerminalText(req.Url))
	fmt.Fprintf(os.Stderr, "Host: %s\n", safeTerminalText(parsed.Host))
	line, ok := input.readLine("Open this URL? [y/N]: ")
	if !ok || !strings.EqualFold(line, "y") && !strings.EqualFold(line, "yes") {
		return declineElicitation(), nil
	}
	if err := openURL(ctx, req.Url); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open URL: %v\n", err)
		return declineElicitation(), nil
	}
	return acp.CreateElicitationResponse{
		Accept: &acp.CreateElicitationAccept{Action: "accept"},
	}, nil
}

func openURL(ctx context.Context, target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd", "/c", "start", "", target)
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", target)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", target)
	}
	return cmd.Start()
}

func declineElicitation() acp.CreateElicitationResponse {
	return acp.CreateElicitationResponse{Decline: &acp.CreateElicitationDecline{Action: "decline"}}
}

func cancelElicitation() acp.CreateElicitationResponse {
	return acp.CreateElicitationResponse{Cancel: &acp.CreateElicitationCancel{Action: "cancel"}}
}

func propertyLabel(name string, property acp.ElicitationPropertySchema) string {
	label := safeTerminalText(name)
	if property.String != nil && property.String.Title != nil {
		label = *property.String.Title
	}
	if property.Number != nil && property.Number.Title != nil {
		label = *property.Number.Title
	}
	if property.Integer != nil && property.Integer.Title != nil {
		label = *property.Integer.Title
	}
	if property.Boolean != nil && property.Boolean.Title != nil {
		label = *property.Boolean.Title
	}
	if property.Array != nil && property.Array.Title != nil {
		label = *property.Array.Title
	}
	if property.String != nil && property.String.Description != nil {
		label += " (" + safeTerminalText(*property.String.Description) + ")"
	}
	return label
}

func propertyDefault(property acp.ElicitationPropertySchema) (acp.ElicitationContentValue, bool, error) {
	switch {
	case property.String != nil && property.String.Default != nil:
		value := acp.ElicitationContentValueString(*property.String.Default)
		return acp.ElicitationContentValue{String: &value}, true, nil
	case property.Number != nil && property.Number.Default != nil:
		value := acp.ElicitationContentValueNumber(*property.Number.Default)
		return acp.ElicitationContentValue{Number: &value}, true, nil
	case property.Integer != nil && property.Integer.Default != nil:
		value := acp.ElicitationContentValueInteger(*property.Integer.Default)
		return acp.ElicitationContentValue{Integer: &value}, true, nil
	case property.Boolean != nil && property.Boolean.Default != nil:
		value := acp.ElicitationContentValueBoolean(*property.Boolean.Default)
		return acp.ElicitationContentValue{Boolean: &value}, true, nil
	case property.Array != nil && property.Array.Default != nil:
		value := acp.ElicitationContentValueStringArray(property.Array.Default)
		return acp.ElicitationContentValue{StringArray: &value}, true, nil
	default:
		return acp.ElicitationContentValue{}, false, nil
	}
}

func parseElicitationValue(property acp.ElicitationPropertySchema, line string) (acp.ElicitationContentValue, error) {
	switch {
	case property.String != nil:
		if !allowedStringValue(*property.String, line) {
			return acp.ElicitationContentValue{}, fmt.Errorf("value is not one of the allowed choices")
		}
		value := acp.ElicitationContentValueString(line)
		return acp.ElicitationContentValue{String: &value}, nil
	case property.Number != nil:
		number, err := strconv.ParseFloat(line, 64)
		if err != nil {
			return acp.ElicitationContentValue{}, err
		}
		if property.Number.Minimum != nil && number < *property.Number.Minimum {
			return acp.ElicitationContentValue{}, fmt.Errorf("value is below the minimum")
		}
		if property.Number.Maximum != nil && number > *property.Number.Maximum {
			return acp.ElicitationContentValue{}, fmt.Errorf("value is above the maximum")
		}
		value := acp.ElicitationContentValueNumber(number)
		return acp.ElicitationContentValue{Number: &value}, nil
	case property.Integer != nil:
		integer, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return acp.ElicitationContentValue{}, err
		}
		if property.Integer.Minimum != nil && integer < *property.Integer.Minimum {
			return acp.ElicitationContentValue{}, fmt.Errorf("value is below the minimum")
		}
		if property.Integer.Maximum != nil && integer > *property.Integer.Maximum {
			return acp.ElicitationContentValue{}, fmt.Errorf("value is above the maximum")
		}
		value := acp.ElicitationContentValueInteger(integer)
		return acp.ElicitationContentValue{Integer: &value}, nil
	case property.Boolean != nil:
		boolean, err := strconv.ParseBool(line)
		if err != nil {
			return acp.ElicitationContentValue{}, err
		}
		value := acp.ElicitationContentValueBoolean(boolean)
		return acp.ElicitationContentValue{Boolean: &value}, nil
	case property.Array != nil:
		items := []string{}
		for _, item := range strings.Split(line, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				items = append(items, item)
			}
		}
		value := acp.ElicitationContentValueStringArray(items)
		return acp.ElicitationContentValue{StringArray: &value}, nil
	default:
		return acp.ElicitationContentValue{}, fmt.Errorf("unsupported property schema")
	}
}

func allowedStringValue(property acp.ElicitationPropertySchemaString, value string) bool {
	if len(property.Enum) == 0 && len(property.OneOf) == 0 {
		return true
	}
	for _, choice := range property.Enum {
		if choice == value {
			return true
		}
	}
	for _, option := range property.OneOf {
		if option.Const == value {
			return true
		}
	}
	return false
}
