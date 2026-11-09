package search

import "testing"

func TestParseClauses(t *testing.T) {
	t.Parallel()

	got := parseClauses("animal -cat width>=300 format=webp type=video ignored~value")
	want := []clause{
		{value: "animal"},
		{value: "cat", negative: true},
		{field: "width", op: ">=", value: "300"},
		{field: "format", op: "=", value: "webp"},
		{field: "type", op: "=", value: "video"},
		{value: "ignored~value"},
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d clauses, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("clause %d: expected %#v, got %#v", i, want[i], got[i])
		}
	}
}

func TestWhereSQLIgnoresInvalidFields(t *testing.T) {
	t.Parallel()

	where, args := WhereSQL("cat unknown=thing width=not-a-number", 1)
	if len(args) != 1 || args[0] != "cat" {
		t.Fatalf("expected only the tag argument, got %v", args)
	}
	if where == "" {
		t.Fatal("expected tag SQL")
	}
}
