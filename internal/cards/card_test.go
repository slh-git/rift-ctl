package cards

import "testing"

func TestParseRiftboundID(t *testing.T) {
	tests := []struct {
		id      string
		set     string
		number  int
		variant string
	}{
		{"unl-060a-219", "UNL", 60, "a"},
		{"ogn-303*-298", "OGN", 303, "*"},
		{"ven-r01", "VEN", 1, ""},
	}

	for _, tt := range tests {
		set, number, variant, err := ParseRiftboundID(tt.id)
		if err != nil {
			t.Fatalf("ParseRiftboundID(%q): %v", tt.id, err)
		}
		if set != tt.set || number != tt.number || variant != tt.variant {
			t.Fatalf("ParseRiftboundID(%q) = %s, %d, %q", tt.id, set, number, variant)
		}
	}
}

func TestDeriveToleratesSpecialIDs(t *testing.T) {
	card := Card{ID: "ven-sp3-006", SetID: "VEN", CollectorNumber: 6}
	if err := card.Derive(); err != nil {
		t.Fatal(err)
	}
	if card.SetID != "VEN" || card.CollectorNumber != 6 {
		t.Fatalf("unexpected derived card: %+v", card)
	}
}
