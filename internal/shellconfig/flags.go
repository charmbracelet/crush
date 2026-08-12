package shellconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"
)

// flagKind is the value type a flag parses from the command line.
type flagKind int

const (
	flagString flagKind = iota
	flagBool
	// flagBoolTrue is a valueless boolean flag (e.g. --think) that stores
	// true when present, without consuming an argument.
	flagBoolTrue
	flagInt
	flagFloat
	// flagKeyValue consumes two args (KEY VALUE) and stores them as a map
	// entry, e.g. --env NAME VALUE.
	flagKeyValue
	// flagJSONObject parses the value as a JSON object (map), rejecting
	// arrays and scalars.
	flagJSONObject
	// flagJSONAny parses the value as arbitrary JSON.
	flagJSONAny
)

// flagOp is how a parsed flag value is written into the target map.
type flagOp int

const (
	// opSet assigns target[jsonKey] = value.
	opSet flagOp = iota
	// opAppend appends value to the []any at target[jsonKey].
	opAppend
	// opSetChild assigns childMap(target, child)[jsonKey] = value, e.g. a
	// single --env KEY VALUE entry under an "env" object.
	opSetChild
	// opMergeChild merges a JSON object into childMap(target, child), e.g.
	// --provider-options '{...}'.
	opMergeChild
)

// flagSpec declares one command-line flag: how it parses, where it writes,
// and an optional validator. A builtin's whole flag surface is a []flagSpec
// handed to applyFlags, which replaces the per-builtin parse loops.
type flagSpec struct {
	name    string // long flag including dashes, e.g. "--api-key"
	jsonKey string // destination key in the target map
	kind    flagKind
	op      flagOp
	child   string // child map name for opSetChild / opMergeChild

	// validate, if non-nil, checks the parsed value before it is stored.
	// It receives the value as string, bool, int64, float64, or
	// map[string]any depending on kind.
	validate func(any) error
}

// applyFlags parses args[start:] against specs and writes the results into
// target. cmd names the invoking command for error messages (e.g.
// "provider add"). An unrecognized flag is an error.
func applyFlags(specs []flagSpec, args []string, start int, target map[string]any, cmd string, stderr io.Writer) error {
	i := start
	for i < len(args) {
		spec, ok := findFlag(specs, args[i])
		if !ok {
			return usage(stderr, fmt.Sprintf("%s: unknown flag %s", cmd, args[i]))
		}

		val, next, err := parseFlagValue(spec, args, i)
		if err != nil {
			return usage(stderr, err.Error())
		}
		if spec.validate != nil {
			if err := spec.validate(val); err != nil {
				return usage(stderr, fmt.Sprintf("%s: %s", cmd, err))
			}
		}
		storeFlag(target, spec, val)
		i = next
	}
	return nil
}

func findFlag(specs []flagSpec, name string) (flagSpec, bool) {
	for _, s := range specs {
		if s.name == name {
			return s, true
		}
	}
	return flagSpec{}, false
}

// parseFlagValue reads the value(s) for spec starting at args[i] and returns
// the parsed value plus the index to resume from.
func parseFlagValue(spec flagSpec, args []string, i int) (any, int, error) {
	name := strings.TrimPrefix(spec.name, "--")

	switch spec.kind {
	case flagString:
		v, err := nextArg(args, i, name)
		if err != nil {
			return nil, 0, err
		}
		return v, i + 2, nil

	case flagBoolTrue:
		return true, i + 1, nil

	case flagBool:
		v, err := nextArg(args, i, name)
		if err != nil {
			return nil, 0, err
		}
		b, err := parseBool(v)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: --%s expects true/false, got %q", args[0], name, v)
		}
		return b, i + 2, nil

	case flagInt:
		v, err := nextArg(args, i, name)
		if err != nil {
			return nil, 0, err
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: --%s expects an integer, got %q", args[0], name, v)
		}
		return n, i + 2, nil

	case flagFloat:
		v, err := nextArg(args, i, name)
		if err != nil {
			return nil, 0, err
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: --%s expects a number, got %q", args[0], name, v)
		}
		return f, i + 2, nil

	case flagKeyValue:
		if i+2 >= len(args) {
			return nil, 0, fmt.Errorf("%s: --%s requires a key and value", args[0], name)
		}
		return [2]string{args[i+1], args[i+2]}, i + 3, nil

	case flagJSONObject:
		v, err := nextArg(args, i, name)
		if err != nil {
			return nil, 0, err
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(v), &object); err != nil || object == nil {
			return nil, 0, fmt.Errorf("%s: --%s expects a JSON object, got %q", args[0], name, v)
		}
		return object, i + 2, nil

	case flagJSONAny:
		v, err := nextArg(args, i, name)
		if err != nil {
			return nil, 0, err
		}
		var parsed any
		if err := json.Unmarshal([]byte(v), &parsed); err != nil {
			return nil, 0, fmt.Errorf("%s: --%s expects valid JSON, got %q: %s", args[0], name, v, err)
		}
		return parsed, i + 2, nil

	default:
		return nil, 0, fmt.Errorf("%s: --%s has unknown flag kind", args[0], name)
	}
}

// nextArg returns args[i+1], erroring if the flag is missing its value.
func nextArg(args []string, i int, flag string) (string, error) {
	if i+1 >= len(args) {
		return "", fmt.Errorf("%s: --%s requires a value", args[0], flag)
	}
	return args[i+1], nil
}

// storeFlag writes a parsed value into target according to spec.op.
func storeFlag(target map[string]any, spec flagSpec, val any) {
	switch spec.op {
	case opSet:
		target[spec.jsonKey] = val
	case opAppend:
		arr, _ := target[spec.jsonKey].([]any)
		target[spec.jsonKey] = append(arr, val)
	case opSetChild:
		if kv, ok := val.([2]string); ok {
			childMap(target, spec.child)[kv[0]] = kv[1]
		}
	case opMergeChild:
		if obj, ok := val.(map[string]any); ok {
			maps.Copy(childMap(target, spec.child), obj)
		}
	}
}
