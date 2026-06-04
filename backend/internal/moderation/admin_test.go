package moderation

import "testing"

func TestParseAdminAccountIDs(t *testing.T) {
	got := ParseAdminAccountIDs(" 1, 2 ,,3 ")
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("parse: got %v", got)
	}
	if len(ParseAdminAccountIDs("")) != 0 {
		t.Fatal("empty should be nil/empty")
	}
}

func TestStaticAdminChecker(t *testing.T) {
	c := NewStaticAdminChecker([]uint{1, 2})
	if !c.IsAdmin(1) || c.IsAdmin(3) || !c.HasAny() {
		t.Fatal("whitelist mismatch")
	}
	empty := NewStaticAdminChecker(nil)
	if empty.IsAdmin(1) || empty.HasAny() {
		t.Fatal("empty checker must deny all")
	}
}
