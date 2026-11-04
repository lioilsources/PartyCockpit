package subtitles

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/subtitles/subtitlestest"
)

func TestSession(t *testing.T) {
	ts := subtitlestest.Fake(t, "tok")
	defer ts.Close()
	lines := make(chan Line, 16)
	c := &Client{URL: ts.URL, Token: "tok"}
	if err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Client{URL: ts.URL, Token: "bad"}).Open(context.Background(), "cs", "en", nil); err == nil {
		t.Fatal("a wrong token must fail")
	}
	s, err := c.Open(context.Background(), "cs", "en", func(l Line) { lines <- l })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Audio(make([]byte, 3200))
	s.End()

	var got []Line
	timeout := time.After(3 * time.Second)
	for len(got) < 4 {
		select {
		case l := <-lines:
			got = append(got, l)
		case <-timeout:
			t.Fatalf("only %d lines: %+v", len(got), got)
		}
	}
	// partial → final keeps one line on screen, then the translation, then done
	if got[0].Final || got[0].Text != "Na zdraví" {
		t.Fatalf("partial %+v", got[0])
	}
	if !got[1].Final || got[1].ID != 1 || got[1].Text != "Na zdraví novomanželů!" {
		t.Fatalf("final %+v", got[1])
	}
	if got[2].Translation != "Cheers to the newlyweds!" || got[2].ToLang != "en" {
		t.Fatalf("translation %+v", got[2])
	}
	if !got[3].Done || !strings.HasPrefix(got[3].Translation, "Cheers") {
		t.Fatalf("done %+v", got[3])
	}
}
