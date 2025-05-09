#!/bin/bash

# Script to check for hidden/invisible characters in Go files
# This helps detect potential prompt injection attempts

echo "Checking Go files for hidden characters..."

# Find all Go files in the repository
go_files=$(find . -name "*.go" -type f)

# Counter for files with hidden characters
files_with_hidden=0

for file in $go_files; do
  # Check for specific Unicode hidden characters that could be used for prompt injection
  # This excludes normal whitespace like tabs and newlines
  # Looking for:
  # - Zero-width spaces (U+200B)
  # - Zero-width non-joiners (U+200C)
  # - Zero-width joiners (U+200D)
  # - Left-to-right/right-to-left marks (U+200E, U+200F)
  # - Bidirectional overrides (U+202A-U+202E)
  # - Byte order mark (U+FEFF)
  if hexdump -C "$file" | grep -E 'e2 80 8b|e2 80 8c|e2 80 8d|e2 80 8e|e2 80 8f|e2 80 aa|e2 80 ab|e2 80 ac|e2 80 ad|e2 80 ae|ef bb bf' > /dev/null 2>&1; then
    echo "Hidden characters found in: $file"
    
    # Show the file with potential issues
    echo "  Hexdump showing suspicious characters:"
    hexdump -C "$file" | grep -E 'e2 80 8b|e2 80 8c|e2 80 8d|e2 80 8e|e2 80 8f|e2 80 aa|e2 80 ab|e2 80 ac|e2 80 ad|e2 80 ae|ef bb bf' | head -10
    
    files_with_hidden=$((files_with_hidden + 1))
  fi
done

if [ $files_with_hidden -eq 0 ]; then
  echo "No hidden characters found in any Go files."
else
  echo "Found hidden characters in $files_with_hidden Go file(s)."
fi

exit $files_with_hidden  # Exit with number of affected files as status code