// find: drop-in wrapper for /usr/bin/find with natural-language mode.
//
//	find -ai "find folder named apiorm"   -> AI builds find command, runs it
//	find <anything else>                  -> exec /usr/bin/find unchanged
//
// Backend: FINDAI_BACKEND=claude (default, uses `claude -p`) or ollama
// (FINDAI_MODEL, default qwen2.5-coder; OLLAMA_HOST, default localhost:11434).
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unicode"
)

const realFind = "/usr/bin/find"

const prompt = `Translate the request into a GNU find command for Linux.
Current directory: %s
Home directory: %s
Rules:
- Search from "." unless the request names a location.
- Add "-not -path '*/.git/*'" style pruning only if useful.
- Reply with ONLY a JSON array of the arguments after "find", no prose, no code fence.
Example: ["." , "-type", "d", "-name", "apiorm"]

Request: %s`

// Actions that change files or run commands need confirmation.
var risky = map[string]bool{"-delete": true, "-exec": true, "-execdir": true, "-ok": true, "-okdir": true,
	"-fprint": true, "-fprint0": true, "-fprintf": true, "-fls": true}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "-ai" {
		die(syscall.Exec(realFind, append([]string{"find"}, os.Args[1:]...), os.Environ()))
	}

	yes, query := false, []string{}
	for _, a := range os.Args[2:] {
		if a == "-y" {
			yes = true
			continue
		}
		query = append(query, a)
	}
	if len(query) == 0 {
		die(fmt.Errorf(`usage: find -ai [-y] "find folder named apiorm"`))
	}

	args, err := translate(strings.Join(query, " "))
	die(err)

	fmt.Fprintf(os.Stderr, "\033[2m$ find %s\033[0m\n", shellQuote(args))
	for _, a := range args {
		if risky[a] && !yes && !confirm() {
			os.Exit(1)
		}
	}
	die(syscall.Exec(realFind, append([]string{"find"}, args...), os.Environ()))
}

func translate(query string) ([]string, error) {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	p := fmt.Sprintf(prompt, cwd, home, query)

	var out string
	var err error
	if os.Getenv("FINDAI_BACKEND") == "ollama" {
		out, err = ollama(p)
	} else {
		out, err = claude(p)
	}
	if err != nil {
		return nil, err
	}

	// Tolerate stray prose or fences around the array.
	i, j := strings.Index(out, "["), strings.LastIndex(out, "]")
	if i < 0 || j < i {
		return nil, fmt.Errorf("bad AI reply: %s", out)
	}
	var args []string
	if err := json.Unmarshal([]byte(out[i:j+1]), &args); err != nil {
		return nil, fmt.Errorf("bad AI reply: %s", out)
	}
	if len(args) > 0 && args[0] == "find" {
		args = args[1:]
	}
	// Non-printable runes (C0/C1 controls, bidi overrides, zero-width) could hide parts of
	// the printed command, e.g. masking -exec.
	for _, a := range args {
		if strings.IndexFunc(a, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
			return nil, fmt.Errorf("AI reply has control characters: %q", a)
		}
	}
	return args, nil
}

func claude(p string) (string, error) {
	// Isolate from the search directory: no project settings/hooks/CLAUDE.md, no tools, no MCP.
	dir, err := os.MkdirTemp("", "findai")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	cmd := exec.Command("claude", "-p", "--model", env("FINDAI_MODEL", "haiku"),
		"--setting-sources", "user", "--tools", "", "--strict-mcp-config", p)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude: %w", err)
	}
	return string(out), nil
}

func ollama(p string) (string, error) {
	body, _ := json.Marshal(map[string]any{"model": env("FINDAI_MODEL", "qwen2.5-coder"), "prompt": p, "stream": false})
	resp, err := http.Post("http://"+env("OLLAMA_HOST", "localhost:11434")+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()
	var r struct{ Response string }
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	return r.Response, nil
}

func confirm() bool {
	fmt.Fprint(os.Stderr, "command modifies files or runs programs. run? [y/N] ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(strings.ToLower(line)) == "y"
}

func shellQuote(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " *?'\"$\\;|&()<>{}[]!#~") {
			a = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
		q[i] = a
	}
	return strings.Join(q, " ")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "find:", err)
		os.Exit(1)
	}
}
