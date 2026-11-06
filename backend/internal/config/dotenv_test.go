package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	body := "# comment\n\nPC_T_A=one\nexport PC_T_B=\"two words\"\nPC_T_C='x=y'\nPC_T_SET=file\nbroken line\nPC_T_EMPTY=\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PC_T_SET", "env")
	for _, k := range []string{"PC_T_A", "PC_T_B", "PC_T_C", "PC_T_EMPTY"} {
		t.Setenv(k, "") // registers cleanup
		os.Unsetenv(k)
	}
	n, err := LoadDotEnv(p)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"PC_T_A": "one", "PC_T_B": "two words", "PC_T_C": "x=y", "PC_T_SET": "env", "PC_T_EMPTY": ""}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if n != 4 {
		t.Errorf("set %d variables, want 4", n)
	}
	if n, err := LoadDotEnv(filepath.Join(t.TempDir(), "missing")); n != 0 || err != nil {
		t.Errorf("missing file: %d, %v", n, err)
	}
}
