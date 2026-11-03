package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 11, 3, 20, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { clock = clock.Add(time.Second); return clock }
	t.Cleanup(func() { s.Close() })
	return s
}

func ids(as []Asset) []string {
	out := []string{}
	for _, a := range as {
		out = append(out, a.ID)
	}
	return out
}

func TestApproveFeedsStageQueueInOrder(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	p, err := s.CreateParty(ctx, "x", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Code) != 5 || p.Mode != "strict" {
		t.Fatalf("party defaults: %+v", p)
	}
	g, _ := s.CreateGuest(ctx, p.Code, "Olin")
	var made []string
	for range 3 {
		a, err := s.CreateAsset(ctx, Asset{PartyCode: p.Code, GuestID: g.ID, Kind: "sticker", Status: "pending"})
		if err != nil {
			t.Fatal(err)
		}
		made = append(made, a.ID)
	}
	// approve out of creation order: queue follows approval order
	for _, i := range []int{2, 0, 1} {
		if err := s.SetAssetStatus(ctx, p.Code, made[i], "approved"); err != nil {
			t.Fatal(err)
		}
	}
	q, _ := s.StageQueue(ctx, p.Code)
	if got, want := ids(q), []string{made[2], made[0], made[1]}; !equal(got, want) {
		t.Fatalf("queue %v want %v", got, want)
	}
	// re-approving is idempotent
	s.SetAssetStatus(ctx, p.Code, made[0], "approved")
	q, _ = s.StageQueue(ctx, p.Code)
	if len(q) != 3 {
		t.Fatalf("duplicate in queue: %v", ids(q))
	}
	// reject removes it from the show
	s.SetAssetStatus(ctx, p.Code, made[2], "rejected")
	q, _ = s.StageQueue(ctx, p.Code)
	if got, want := ids(q), []string{made[0], made[1]}; !equal(got, want) {
		t.Fatalf("after reject %v want %v", got, want)
	}
	// reorder
	if err := s.ReorderQueue(ctx, p.Code, []string{made[1], made[0]}); err != nil {
		t.Fatal(err)
	}
	q, _ = s.StageQueue(ctx, p.Code)
	if got, want := ids(q), []string{made[1], made[0]}; !equal(got, want) {
		t.Fatalf("after reorder %v want %v", got, want)
	}
}

func TestSourcesNeverReachTheStage(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	p, _ := s.CreateParty(ctx, "x", time.Hour)
	src, _ := s.CreateAsset(ctx, Asset{PartyCode: p.Code, Kind: "upload", Status: "source"})
	if err := s.SetAssetStatus(ctx, p.Code, src.ID, "approved"); err != ErrNotFound {
		t.Fatalf("approving a source photo: err=%v, want ErrNotFound", err)
	}
	as, _ := s.Assets(ctx, p.Code)
	if len(as) != 0 {
		t.Fatalf("source listed for moderation: %v", ids(as))
	}
}

func TestExpiredParties(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	p, _ := s.CreateParty(ctx, "x", time.Hour)
	if c, _ := s.ExpiredParties(ctx, 0); len(c) != 0 {
		t.Fatalf("fresh party expired: %v", c)
	}
	base := s.Now()
	s.Now = func() time.Time { return base.Add(8 * 24 * time.Hour) }
	c, _ := s.ExpiredParties(ctx, 7*24*time.Hour)
	if len(c) != 1 || c[0] != p.Code {
		t.Fatalf("expired = %v", c)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
