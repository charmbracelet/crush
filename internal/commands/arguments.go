package commands

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	// positionalArgPattern matches positional arguments like $1, $2, $3, etc.
	// Matches $ followed by one or more digits
	positionalArgPattern = regexp.MustCompile(`\$(\d+)`)

	// allArgumentsPattern matches $ARGS or $ARGUMENTS placeholder
	// Matches either $ARGS or $ARGUMENTS (case-sensitive)
	allArgumentsPattern = regexp.MustCompile(`\$(?:ARGS|ARGUMENTS)`)
)

// hasArgumentPlaceholders checks if the content contains any argument placeholders.
//
// Returns true if the content contains $ARGS, $ARGUMENTS, or any positional argument ($1, $2, etc.).
func hasArgumentPlaceholders(content string) bool {
	return allArgumentsPattern.MatchString(content) || positionalArgPattern.MatchString(content)
}

// hasAllRequiredArguments checks if the content references all required arguments.
//
// Parameters:
//   - content: The command content (may contain $ARGS, $ARGUMENTS, $1, $2, etc.)
//   - requiredCount: The number of arguments required by the command
//
// Returns true if:
//   - Content contains $ARGS or $ARGUMENTS (which covers all arguments), OR
//   - Content contains all positional arguments from $1 to $requiredCount
//
// Returns false if some required arguments are missing from the content.
func hasAllRequiredArguments(content string, requiredCount int) bool {
	// If $ARGS or $ARGUMENTS is present, all arguments are covered
	if allArgumentsPattern.MatchString(content) {
		return true
	}

	// If no arguments required, nothing to check
	if requiredCount <= 0 {
		return true
	}

	// Check if all positional arguments from $1 to $requiredCount are present
	foundArgs := make(map[int]bool)
	matches := positionalArgPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		pos, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if pos >= 1 && pos <= requiredCount {
			foundArgs[pos] = true
		}
	}

	// Check if we found all required arguments
	return len(foundArgs) == requiredCount
}

// RequiredArguments represents the argument requirements for a command.
type RequiredArguments struct {
	// HasAllArguments is true if the command uses $ARGUMENTS placeholder
	HasAllArguments bool

	// MaxPositional is the highest positional argument number found (e.g., $1, $2, $3 → MaxPositional = 3)
	// 0 if no positional arguments found
	MaxPositional int

	// RequiredCount is the number of arguments required
	// If HasAllArguments is true, this is set to -1 (variable number)
	// Otherwise, this is MaxPositional
	RequiredCount int
}

// extractRequiredArguments analyzes command content and argument-hint to identify which arguments are needed.
//
// The function detects arguments from two sources (in order of precedence):
//   1. Content placeholders: $ARGS, $ARGUMENTS, $1, $2, $3, etc.
//   2. ArgumentHint frontmatter: "[arg1] [arg2]" format
//
// Examples:
//   - Content with "$ARGS" or "$ARGUMENTS" → HasAllArguments=true, RequiredCount=-1
//   - Content with "$1 $2" → MaxPositional=2, RequiredCount=2
//   - Content with "$1 $3" (missing $2) → MaxPositional=3, RequiredCount=3 (all positions up to max)
//   - ArgumentHint "[number1] [number2]" → RequiredCount=2
//   - Content with no placeholders and no ArgumentHint → RequiredCount=0
//
// Parameters:
//   - content: The command content (may contain $ARGS, $ARGUMENTS, $1, $2, etc.)
//   - argumentHint: The argument-hint from frontmatter (e.g., "[arg1] [arg2]")
//
// Returns a RequiredArguments struct describing the argument requirements.
func extractRequiredArguments(content string, argumentHint string) RequiredArguments {
	result := RequiredArguments{}

	// Check for $ARGS or $ARGUMENTS placeholder
	if allArgumentsPattern.MatchString(content) {
		result.HasAllArguments = true
		result.RequiredCount = -1 // Variable number of arguments
		return result
	}

	// Find all positional argument references ($1, $2, etc.)
	matches := positionalArgPattern.FindAllStringSubmatch(content, -1)
	contentArgCount := 0
	if len(matches) > 0 {
		// Find the maximum positional argument number
		maxPos := 0
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			pos, err := strconv.Atoi(match[1])
			if err != nil {
				continue
			}
			if pos > maxPos {
				maxPos = pos
			}
		}
		result.MaxPositional = maxPos
		contentArgCount = maxPos // Content requires all positions up to max
	}

	// Check argument-hint
	hintArgCount := 0
	if argumentHint != "" {
		hintArgCount = countArgumentsFromHint(argumentHint)
	}

	// Use the maximum of content and hint requirements
	// This ensures if hint says 2 args but content only has $1, we still require 2
	if hintArgCount > contentArgCount {
		result.RequiredCount = hintArgCount
	} else {
		result.RequiredCount = contentArgCount
	}

	return result
}

// countArgumentsFromHint counts the number of arguments from an argument-hint string.
//
// The argument-hint format uses square brackets to indicate arguments:
//   - "[arg1] [arg2]" → 2 arguments
//   - "[number1] [number2] [number3]" → 3 arguments
//   - "[pr-number]" → 1 argument
//
// Returns the number of arguments found, or 0 if none.
func countArgumentsFromHint(hint string) int {
	if hint == "" {
		return 0
	}

	// Count occurrences of [...] patterns
	// This regex matches [ followed by any characters (non-greedy) followed by ]
	argPattern := regexp.MustCompile(`\[[^\]]+\]`)
	matches := argPattern.FindAllString(hint, -1)
	return len(matches)
}

// substituteArguments substitutes argument placeholders in command content with actual argument values.
//
// Supported placeholders:
//   - $ARGS or $ARGUMENTS: Replaced with all arguments joined by a single space
//   - $1, $2, $3, etc.: Replaced with the corresponding positional argument
//
// Examples:
//   - Content: "Review PR $ARGS", args: ["123", "high"] → "Review PR 123 high"
//   - Content: "Review PR $ARGUMENTS", args: ["123", "high"] → "Review PR 123 high"
//   - Content: "Review PR $1 with priority $2", args: ["123", "high"] → "Review PR 123 with priority high"
//   - Content: "Use $1 and $3", args: ["a", "b", "c"] → "Use a and c"
//
// If a positional argument is missing (e.g., $3 but only 2 args), it's replaced with an empty string.
// If $ARGS or $ARGUMENTS is present, all occurrences are replaced, and positional substitutions still occur.
//
// Returns the content with all placeholders substituted.
func substituteArguments(content string, args []string) string {
	if len(args) == 0 {
		// No arguments provided - replace all placeholders with empty strings
		result := allArgumentsPattern.ReplaceAllString(content, "")
		result = positionalArgPattern.ReplaceAllStringFunc(result, func(match string) string {
			return ""
		})
		return result
	}

	// First, replace $ARGS or $ARGUMENTS with all arguments joined
	allArgsStr := strings.Join(args, " ")
	result := allArgumentsPattern.ReplaceAllString(content, allArgsStr)

	// Then replace positional arguments ($1, $2, etc.)
	result = positionalArgPattern.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the number from $N
		numStr := match[1:] // Remove the $
		pos, err := strconv.Atoi(numStr)
		if err != nil {
			return match // Invalid format, return unchanged
		}

		// Convert to 0-based index
		index := pos - 1

		// Check if argument exists
		if index >= 0 && index < len(args) {
			return args[index]
		}

		// Argument doesn't exist, return empty string
		return ""
	})

	return result
}

