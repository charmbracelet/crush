package diffdetect

import "testing"

func TestInspect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    Signal
	}{
		{
			name: "git unified diff",
			content: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,2 +1,3 @@
 package main
+import "fmt"
`,
			want: Signal{HasHunk: true, HasFileHeader: true, HasGitHeader: true},
		},
		{
			name: "non-git unified diff",
			content: `--- old.c
+++ old.c
@@ -1 +1 @@
-old
+new
`,
			want: Signal{HasHunk: true, HasFileHeader: true, HasGitHeader: false},
		},
		{
			name: "plain text",
			content: "hello world",
			want: Signal{},
		},
		{
			name: "hunk only",
			content: "@@ -1 +1 @@\n-old\n+new\n",
			want: Signal{HasHunk: true},
		},
		{
			name: "headers only",
			content: "--- a/file\n+++ b/file\n",
			want: Signal{HasFileHeader: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Inspect(tt.content)
			if got != tt.want {
				t.Errorf("Inspect() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIsUnifiedDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "github-style multi-file diff",
			content: `diff --git a/one.txt b/one.txt
--- a/one.txt
+++ b/one.txt
@@ -1 +1 @@
-a
+b
diff --git a/two.txt b/two.txt
--- a/two.txt
+++ b/two.txt
@@ -1 +1 @@
-c
+d
`,
			want: true,
		},
		{
			name: "non-git unified patch",
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
			name: "new file from dev null",
			content: `--- /dev/null
+++ newfile.txt
@@ -0,0 +1,2 @@
+hello
+world
`,
			want: true,
		},
		{
			name: "markdown false positive candidate",
			content: `- Item one
- Item two
+ Bonus item
- Item three
`,
			want: false,
		},
		{
			name: "headers without hunk",
			content: `--- a/somefile.txt
+++ b/somefile.txt
Just some content here
No hunk markers at all
`,
			want: false,
		},
		{
			name: "hunk without headers",
			content: `@@ -1,3 +1,4 @@
 some line
-another line
+changed line
`,
			want: false,
		},
		{
			name:    "empty",
			content: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsUnifiedDiff(tt.content); got != tt.want {
				t.Errorf("IsUnifiedDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}
