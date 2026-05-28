package formatting

import "testing"

func TestToLowerASCII(t *testing.T) {
	if got := ToLower("WiSeR-15Q"); got != "wiser-15q" {
		t.Fatalf("expected wiser-15q, got %q", got)
	}
}

func TestToLowerPreservesMultibyte(t *testing.T) {
	// Rendered list rows contain emoji status/type icons. The previous
	// implementation indexed a []rune by byte offset, corrupting any string
	// with multi-byte runes (inserting NUL runes). Verify round-trip integrity.
	in := "◆ 🔨 WISER-15Q Café"
	want := "◆ 🔨 wiser-15q café"
	if got := ToLower(in); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestContainsCaseInsensitiveWithEmojiAndID(t *testing.T) {
	// Mirrors an actual rendered issue row: color markup + emoji + full ID.
	row := "  [#ff0000]◆[-] 🔨 wiser-15q [P2] ExpiringPlansQueryTest needs rewrite"
	cases := []struct {
		query string
		want  bool
	}{
		{"15q", true},
		{"WISER-15Q", true},
		{"rewrite", true},
		{"🔨", true},
		{"nope", false},
	}
	for _, tc := range cases {
		if got := ContainsCaseInsensitive(row, tc.query); got != tc.want {
			t.Errorf("ContainsCaseInsensitive(row, %q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}
