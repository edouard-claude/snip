package hook

import "testing"

func TestExtractFirstSegment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "git log -10", "git log -10"},
		{"semicolon", "git add . ; git commit", "git add . "},
		{"pipe", "git log | head -5", "git log "},
		{"and", "git add . && git commit -m \"msg\"", "git add . "},
		{"quoted semicolon", `git commit -m "hello; world"`, `git commit -m "hello; world"`},
		{"quoted pipe", `echo "foo | bar" && next`, `echo "foo | bar" `},
		{"single quoted", "git commit -m 'hello; world'", "git commit -m 'hello; world'"},
		{"escaped quote", `git commit -m "say \"hi\""`, `git commit -m "say \"hi\""`},
		{"heredoc multiline", "git commit -m \"$(cat <<'EOF'\nfix: something\nEOF\n)\"", "git commit -m \"$(cat <<'EOF'"},
		{"empty", "", ""},
		{"whitespace only", "   ", "   "},
		{"newline first", "git\nstatus", "git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFirstSegment(tt.input)
			if got != tt.want {
				t.Errorf("ExtractFirstSegment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseSegment(t *testing.T) {
	tests := []struct {
		name       string
		segment    string
		wantPrefix string
		wantEnv    string
		wantBare   string
	}{
		{"simple", "git log -10", "", "", "git log -10"},
		{"leading space", "  git status", "  ", "", "git status"},
		{"trailing space", "git status  ", "", "", "git status  "},
		{"both space", "  git status  ", "  ", "", "git status  "},
		{"single env", "CGO_ENABLED=0 go test ./...", "", "CGO_ENABLED=0 ", "go test ./..."},
		{"multi env", "FOO=1 BAR=2 make build", "", "FOO=1 BAR=2 ", "make build"},
		{"env with leading space", "  CGO_ENABLED=0 go test", "  ", "CGO_ENABLED=0 ", "go test"},
		{"no env - equals in arg", "git log --format=oneline", "", "", "git log --format=oneline"},
		{"escaped quote in env value", `MSG="say \"hi\"" go test`, "", `MSG="say \"hi\"" `, "go test"},
		{"empty", "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, envVars, bareCmd := ParseSegment(tt.segment)
			if prefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", prefix, tt.wantPrefix)
			}
			if envVars != tt.wantEnv {
				t.Errorf("envVars = %q, want %q", envVars, tt.wantEnv)
			}
			if bareCmd != tt.wantBare {
				t.Errorf("bareCmd = %q, want %q", bareCmd, tt.wantBare)
			}
		})
	}
}

func TestBaseCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "git log -10", "git"},
		{"no args", "git", "git"},
		{"leading space", "  git status", "git"},
		{"tab separated", "make\tbuild", "make"},
		{"empty", "", ""},
		{"path", "/usr/bin/git status", "/usr/bin/git"},
		{"quoted path", `"/usr/local/bin/snip" -- git`, `"/usr/local/bin/snip"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BaseCommand(tt.input)
			if got != tt.want {
				t.Errorf("BaseCommand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
