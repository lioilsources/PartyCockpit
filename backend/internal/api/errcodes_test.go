package api

import (
	"encoding/json"
	"os"
	"testing"
)

// Every error code the server sends must have a translation key in the Czech
// (source) and English (fallback) catalogs of the frontend.
func TestErrCodesHaveCatalogKeys(t *testing.T) {
	codes := map[string]bool{"nick_taken": true}
	for _, c := range errCodes {
		codes[c] = true
	}
	for _, lang := range []string{"cs", "en"} {
		raw, err := os.ReadFile("../../../frontend/src/i18n/" + lang + ".json")
		if err != nil {
			t.Skipf("frontend catalog not available: %v", err)
		}
		var cat map[string]string
		if err := json.Unmarshal(raw, &cat); err != nil {
			t.Fatal(err)
		}
		for c := range codes {
			if cat["err."+c] == "" {
				t.Errorf("%s.json has no err.%s", lang, c)
			}
		}
	}
}
