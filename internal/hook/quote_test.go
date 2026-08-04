package hook

import "testing"

// TestQuoteBinFor pins issue #150: on Windows the rewritten command carried a
// Go string literal, so the executable path arrived with doubled backslashes
// and wrapped in quotes that PowerShell reads as a string expression
// ("Unexpected token 'run' in expression or statement").
func TestQuoteBinFor(t *testing.T) {
	cases := []struct {
		name string
		path string
		goos string
		want string
	}{
		{
			name: "windows path without spaces is bare",
			path: `C:\Users\Administrator\.snip\snip.exe`,
			goos: "windows",
			want: `C:\Users\Administrator\.snip\snip.exe`,
		},
		{
			// Quotes are still required, and the backslashes must survive
			// verbatim: "C:\\Program Files\\..." names no existing file.
			name: "windows path with a space keeps its backslashes",
			path: `C:\Program Files\snip\snip.exe`,
			goos: "windows",
			want: `"C:\Program Files\snip\snip.exe"`,
		},
		{
			name: "posix path is quoted",
			path: "/usr/local/bin/snip",
			goos: "darwin",
			want: `"/usr/local/bin/snip"`,
		},
		{
			name: "posix path with a space is quoted",
			path: "/Users/John Doe/bin/snip",
			goos: "linux",
			want: `"/Users/John Doe/bin/snip"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QuoteBinFor(tc.path, tc.goos); got != tc.want {
				t.Errorf("QuoteBinFor(%q, %q) = %q, want %q", tc.path, tc.goos, got, tc.want)
			}
		})
	}
}

// TestRewriteCommandWindowsBin is the end-to-end shape Codex receives: the
// command must start with the executable, not with a quoted string.
func TestRewriteCommandWindowsBin(t *testing.T) {
	const bin = `C:\Users\Administrator\.snip\snip.exe`
	cmdSet := map[string]struct{}{"git": {}}

	got, known, _ := rewriteGroup("git diff --check", cmdSet, nil, QuoteBinFor(bin, "windows"), bin)
	want := `C:\Users\Administrator\.snip\snip.exe run -- git diff --check`
	if got != want {
		t.Errorf("rewritten = %q, want %q", got, want)
	}
	if !known {
		t.Error("expected the git base to be known")
	}

	// Re-running the hook on its own output must not wrap it twice.
	again, _, _ := rewriteGroup(want, cmdSet, nil, QuoteBinFor(bin, "windows"), bin)
	if again != want {
		t.Errorf("second pass = %q, want it unchanged", again)
	}
}
