package providers

import "testing"

func TestRetiredStylesAreNeverOffered(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for id := range Retired {
		if _, ok := c.Style(id); ok {
			t.Fatalf("retired style %q is still in the catalog", id)
		}
	}
	for _, s := range c.Public()["styles"].([]PublicStyle) {
		if Retired[s.ID] {
			t.Fatalf("retired style %q offered to guests", s.ID)
		}
	}
	if len(c.Styles) < 50 {
		t.Fatalf("catalog shrank to %d styles", len(c.Styles))
	}
}
