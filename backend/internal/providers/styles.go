package providers

import (
	_ "embed"
	"encoding/json"
)

// restyle_styles.json is generated from Restyler's catalog
// (scripts/sync-restyler-styles.mjs); do not hand-edit it.
//
//go:embed restyle_styles.json
var stylesJSON []byte

type Style struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Group string `json:"group"`
	Block string `json:"block"`
}

type Catalog struct {
	Medium   map[string]string `json:"medium"`
	Negative map[string]string `json:"negative"`
	Groups   []string          `json:"groups"`
	Styles   []Style           `json:"styles"`
	byID     map[string]Style
}

// Retired styles: on restyle_flux they turned a clothed selfie into a nude
// (classical nudes, pin-ups; FLUX ignores the negative prompt). Restyler
// retires them too (kRetiredRestyleStyleIds); the list is kept here as well,
// so a regenerated restyle_styles.json can never bring them back.
var Retired = map[string]bool{
	"davinci-chalk": true, "schiele": true, "greek": true, "greek-vase-red": true,
	"cycladic": true, "himba": true, "pin-up-50s": true, "titian": true,
	"rubens": true, "renoir": true, "zorn": true, "david-neoclassical": true,
	"frazetta": true, "sorayama": true, "bacon": true, "botticelli": true,
	"manet": true, "ingres": true,
}

func LoadCatalog() (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(stylesJSON, &c); err != nil {
		return nil, err
	}
	kept := c.Styles[:0]
	for _, s := range c.Styles {
		if !Retired[s.ID] {
			kept = append(kept, s)
		}
	}
	c.Styles = kept
	c.byID = make(map[string]Style, len(c.Styles))
	for _, s := range c.Styles {
		c.byID[s.ID] = s
	}
	return &c, nil
}

func (c *Catalog) Style(id string) (Style, bool) {
	s, ok := c.byID[id]
	return s, ok
}

// Medium is "photo" or "illustration"; anything else falls back to illustration.
func NormMedium(m string) string {
	if m == "photo" {
		return "photo"
	}
	return "illustration"
}

// Prompt is Restyler's restylePrompt: the medium sentence first, the style block
// last (CLIP/T5 weight early tokens most, and the medium must not be argued with).
func (c *Catalog) Prompt(s Style, medium string) string {
	return c.Medium[NormMedium(medium)] + ", " + s.Block
}

func (c *Catalog) NegativePrompt(medium string) string { return c.Negative[NormMedium(medium)] }

// Public is what the guest picker gets (no prompt text).
type PublicStyle struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Group string `json:"group"`
}

func (c *Catalog) Public() map[string]any {
	out := make([]PublicStyle, 0, len(c.Styles))
	for _, s := range c.Styles {
		out = append(out, PublicStyle{s.ID, s.Label, s.Group})
	}
	return map[string]any{"groups": c.Groups, "styles": out}
}
