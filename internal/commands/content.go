package commands

// processCommandContent processes command content by substituting arguments while preserving file references.
//
// The function:
//   - Substitutes $ARGS, $ARGUMENTS, and $1, $2, etc. with provided arguments
//   - Preserves @filename references in the output (they are handled separately)
//   - Handles missing arguments gracefully (replaces with empty string)
//
// Parameters:
//   - content: The command content (may contain $ARGS, $ARGUMENTS, $1, $2, etc., and @filename)
//   - args: The arguments provided by the user
//
// Returns the processed content with all argument placeholders substituted.
// File references (@filename) remain in the output for separate processing.
func processCommandContent(content string, args []string) string {
	// Substitute arguments in the content
	// This replaces $ARGS, $ARGUMENTS, and $1, $2, etc. with actual argument values
	processed := substituteArguments(content, args)

	// File references (@filename) are preserved in the output
	// They will be extracted and processed separately by parseFileReferences
	return processed
}

