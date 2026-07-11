package cards

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatPretty(t *testing.T) {
	energy, might := 1, 3
	card := Card{
		ID:              "unl-060a-219",
		Name:            "Accelerate",
		SetID:           "UNL",
		CollectorNumber: 60,
		Variant:         "a",
		Type:            "Spell",
		Domains:         []string{"Fury"},
		Rarity:          "Common",
		Energy:          &energy,
		Might:           &might,
		TextPlain:       "Deal 2 to a unit.",
		Flavour:         "Faster.",
		Artist:          "Someone",
	}
	var buf bytes.Buffer
	if err := FormatPretty(&buf, card); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Accelerate", "UNL-060a", "Spell", "Fury", "Energy 1", "Deal 2 to a unit.", "Faster.", "Artist: Someone"} {
		if !strings.Contains(out, want) {
			t.Fatalf("pretty output missing %q:\n%s", want, out)
		}
	}
}

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
