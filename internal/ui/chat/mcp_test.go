package chat

import (
	"strings"
	"testing"
)

func TestLooksLikeDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "simple unified diff",
			content: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main
 
+import "fmt"
+
 func main() {
-    println("hello")
+    fmt.Println("hello")
 }
`,
			want: true,
		},
		{
			name:    "plain text",
			content: "This is just some plain text with no diff markers.",
			want:    false,
		},
		{
			name:    "empty string",
			content: "",
			want:    false,
		},
		{
			name: "markdown with headers",
			content: `# Title

Some content here.

## Subtitle

More content with **bold** text.
`,
			want: false,
		},
		{
			name: "diff with mixed content",
			content: `diff --git a/file.txt b/file.txt
--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-old line
+new line
`,
			want: true,
		},
		{
			name: "only plus/minus without hunk or headers",
			content: `Hello world
---
This is not really a diff
Just some text with a few symbols
+ another line
More regular content here
And even more content
`,
			want: false,
		},
		{
			name: "GitHub PR diff format",
			content: `diff --git a/src/app.ts b/src/app.ts
index abc1234..def5678 100644
--- a/src/app.ts
+++ b/src/app.ts
@@ -10,6 +10,8 @@ function handleRequest() {
   const data = getData();
+  validate(data);
+  log(data);
   return process(data);
 }
`,
			want: true,
		},
		{
			name: "non-git unified patch with hunk and headers",
			content: `--- a/old.c
+++ b/old.c
@@ -1,3 +1,4 @@
 #include <stdio.h>
-int main() {
+int main(int argc, char **argv) {
     return 0;
 }
`,
			want: true,
		},
		{
			name: "file headers without hunk markers",
			content: `--- a/somefile.txt
+++ b/somefile.txt
Just some content here
No hunk markers at all
`,
			want: false,
		},
		{
			name: "hunk markers without file headers",
			content: `@@ -1,3 +1,4 @@
 some line
-another line
+changed line
`,
			want: false,
		},
		{
			name: "markdown list with plus signs",
			content: `- Item one
- Item two
+ Bonus item
- Item three
`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := looksLikeDiff(tt.content)
			if got != tt.want {
				t.Errorf("looksLikeDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseUnifiedDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []parsedDiffFile
	}{
		{
			name: "simple diff with additions and removals",
			input: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main
 
+import "fmt"
+
 func main() {
-    println("hello")
+    fmt.Println("hello")
 }
`,
			want: []parsedDiffFile{
				{
					path:   "main.go",
					before: "package main\n\nfunc main() {\n    println(\"hello\")\n}",
					after:  "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"hello\")\n}",
				},
			},
		},
		{
			name: "new file creation",
			input: `diff --git a/newfile.go b/newfile.go
new file mode 100644
--- /dev/null
+++ b/newfile.go
@@ -0,0 +1,3 @@
+package main
+
+func main() {}
`,
			want: []parsedDiffFile{
				{
					path:   "newfile.go",
					before: "",
					after:  "package main\n\nfunc main() {}",
				},
			},
		},
		{
			name: "file deletion",
			input: `diff --git a/oldfile.go b/oldfile.go
deleted file mode 100644
--- a/oldfile.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package main
-
-func main() {}
`,
			want: []parsedDiffFile{
				{
					path:   "oldfile.go",
					before: "package main\n\nfunc main() {}",
					after:  "",
				},
			},
		},
		{
			name:  "non-diff content",
			input: "Just some regular text",
			want:  nil,
		},
		{
			name: "diff with timestamp in header",
			input: `diff --git a/config.yml b/config.yml
--- a/config.yml	2024-01-15 10:30:00
+++ b/config.yml	2024-01-15 10:31:00
@@ -1,3 +1,4 @@
 name: myapp
-version: 1.0
+version: 1.1
+debug: true
`,
			want: []parsedDiffFile{
				{
					path:   "config.yml",
					before: "name: myapp\nversion: 1.0",
					after:  "name: myapp\nversion: 1.1\ndebug: true",
				},
			},
		},
		{
			name: "multi-file diff",
			input: `diff --git a/one.txt b/one.txt
--- a/one.txt
+++ b/one.txt
@@ -1,3 +1,3 @@
 line one
-line two
+line two updated
 line three
diff --git a/two.txt b/two.txt
--- a/two.txt
+++ b/two.txt
@@ -1,2 +1,3 @@
 alpha
+beta
 gamma
`,
			want: []parsedDiffFile{
				{
					path:   "one.txt",
					before: "line one\nline two\nline three",
					after:  "line one\nline two updated\nline three",
				},
				{
					path:   "two.txt",
					before: "alpha\ngamma",
					after:  "alpha\nbeta\ngamma",
				},
			},
		},
		{
			name: "non-git unified patch",
			input: `--- old.c
+++ old.c
@@ -1,3 +1,4 @@
 #include <stdio.h>
-int main() {
+int main(int argc, char **argv) {
     return 0;
 }
`,
			want: []parsedDiffFile{
				{
					path:   "old.c",
					before: "#include <stdio.h>\nint main() {\n    return 0;\n}",
					after:  "#include <stdio.h>\nint main(int argc, char **argv) {\n    return 0;\n}",
				},
			},
		},
		{
			name: "non-git new file from /dev/null",
			input: `--- /dev/null
+++ newfile.txt
@@ -0,0 +1,2 @@
+hello
+world
`,
			want: []parsedDiffFile{
				{
					path:   "newfile.txt",
					before: "",
					after:  "hello\nworld",
				},
			},
		},
		{
			name: "non-git new file with only +++ header",
			input: `+++ brand_new.go
@@ -0,0 +1,3 @@
+package main
+
+func main() {}
`,
			want: []parsedDiffFile{
				{
					path:   "brand_new.go",
					before: "",
					after:  "package main\n\nfunc main() {}",
				},
			},
		},
		{
			name: "multi-hunk single file",
			input: `diff --git a/big.go b/big.go
--- a/big.go
+++ b/big.go
@@ -1,4 +1,5 @@
 package main
+import "os"
 
 func init() {
@@ -10,3 +11,3 @@
-    println("done")
+    fmt.Println("done")
 }
`,
			want: []parsedDiffFile{
				{
					path:   "big.go",
					before: "package main\n\nfunc init() {\n    println(\"done\")\n}",
					after:  "package main\nimport \"os\"\n\nfunc init() {\n    fmt.Println(\"done\")\n}",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseUnifiedDiff(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseUnifiedDiff() returned %d files, want %d", len(got), len(tt.want))
				return
			}
			for i, w := range tt.want {
				if got[i].path != w.path {
					t.Errorf("parseUnifiedDiff()[%d].path = %q, want %q", i, got[i].path, w.path)
				}
				if got[i].before != w.before {
					t.Errorf("parseUnifiedDiff()[%d].before = %q, want %q", i, got[i].before, w.before)
				}
				if got[i].after != w.after {
					t.Errorf("parseUnifiedDiff()[%d].after = %q, want %q", i, got[i].after, w.after)
				}
			}
		})
	}
}

func TestLooksLikeDiffVersusMarkdown(t *testing.T) {
	t.Parallel()

	// A unified diff should be detected as a diff, not markdown,
	// even though it contains "-" which could match markdown patterns.
	diffContent := strings.Join([]string{
		"diff --git a/README.md b/README.md",
		"--- a/README.md",
		"+++ b/README.md",
		"@@ -1,3 +1,3 @@",
		" # Title",
		"-Old subtitle",
		"+New subtitle",
		" Some content",
	}, "\n")

	if !looksLikeDiff(diffContent) {
		t.Error("looksLikeDiff() should detect unified diff")
	}
}
