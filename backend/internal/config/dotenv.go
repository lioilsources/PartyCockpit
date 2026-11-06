package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv sets KEY=VALUE lines from path as environment variables, so the
// release binary runs on a laptop (Windows too) without a shell wrapper.
// Variables already set win, as with `set -a; . ./.env` in make run.
// A missing file is not an error. Returns how many variables it set.
func LoadDotEnv(path string) (int, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		if _, set := os.LookupEnv(k); set {
			continue
		}
		if err := os.Setenv(k, v); err != nil {
			return n, err
		}
		n++
	}
	return n, sc.Err()
}
