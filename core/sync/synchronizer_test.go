package sync

import "testing"

func TestDiffRoot(t *testing.T) {
	s := NewSynchronizer()
	s.Diff("player", map[string]any{"nickname": "a"})
	first := s.Take()
	if first["player.nickname"] != `"a"` {
		t.Fatalf("first diff = %#v", first)
	}
	s.Diff("player", map[string]any{"nickname": "a"})
	if again := s.Take(); len(again) != 0 {
		t.Fatalf("unchanged root produced diff %#v", again)
	}
	s.Diff("player", map[string]any{})
	deleted := s.Take()
	if deleted["player.nickname"] != "null" {
		t.Fatalf("delete diff = %#v", deleted)
	}
}
