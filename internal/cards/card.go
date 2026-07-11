package cards

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Card mirrors the Riftcodex Card payload, plus a few local derived fields.
type Card struct {
	ID                string
	RiftcodexID       string
	Name              string
	TCGPlayerID       string
	CollectorNumber   int
	Orientation       string
	Energy            *int
	Might             *int
	Power             *int
	Type              string
	Supertype         string
	Rarity            string
	Domains           []string
	TextPlain         string
	TextRich          string
	Flavour           string
	SetID             string
	SetLabel          string
	ImageURL          string
	Artist            string
	AccessibilityText string
	Tags              []string
	CleanName         string
	SourceUpdatedAt   string
	AlternateArt      bool
	Overnumbered      bool
	Signature         bool
	New               bool
	APIJSON           string

	Variant   string
	Faction   string
	ImagePath string
	UpdatedAt time.Time
}

// Ref identifies a card by set, collector number, and optional variant suffix.
type Ref struct {
	SetID           string
	CollectorNumber int
	Variant         string
	IsRune          bool
}

var (
	riftboundIDPattern = regexp.MustCompile(`^([a-z]+)-(\d+)([a-z*]?)-(\d+)$`)
	runeIDPattern      = regexp.MustCompile(`^([a-z]+)-r(\d+)$`)
	shortRefPattern    = regexp.MustCompile(`^([A-Z]{3})-((?:R)?\d{1,3})([a-z*]?)$`)
)

// Derive fills local fields from API fields.
func (c *Card) Derive() error {
	setID, number, variant, err := ParseRiftboundID(c.ID)
	c.SetID = strings.ToUpper(c.SetID)
	if err == nil && c.SetID == "" {
		c.SetID = setID
	}
	if err == nil && c.CollectorNumber == 0 {
		c.CollectorNumber = number
	}
	if err == nil {
		c.Variant = variant
	}
	c.Faction = strings.Join(c.Domains, ", ")
	return nil
}

// ParseRiftboundID parses IDs like unl-060a-219 or ogn-303*-298.
func ParseRiftboundID(id string) (string, int, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(id))
	m := riftboundIDPattern.FindStringSubmatch(normalized)
	if m == nil {
		rune := runeIDPattern.FindStringSubmatch(normalized)
		if rune == nil {
			return "", 0, "", fmt.Errorf("invalid riftbound id: %q", id)
		}
		n, err := strconv.Atoi(rune[2])
		if err != nil {
			return "", 0, "", err
		}
		return strings.ToUpper(rune[1]), n, "", nil
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, "", err
	}
	return strings.ToUpper(m[1]), n, m[3], nil
}

// ParseShortRef parses short refs like UNL-181 or UNL-060a.
func ParseShortRef(s string) (Ref, error) {
	s = strings.TrimSpace(s)
	m := shortRefPattern.FindStringSubmatch(strings.ToUpper(s))
	if m == nil {
		return Ref{}, fmt.Errorf("invalid short ref: %q", s)
	}
	numPart := m[2]
	isRune := strings.HasPrefix(numPart, "R")
	if isRune {
		numPart = strings.TrimPrefix(numPart, "R")
	}
	n, err := strconv.Atoi(numPart)
	if err != nil {
		return Ref{}, err
	}
	return Ref{SetID: m[1], CollectorNumber: n, Variant: strings.ToLower(m[3]), IsRune: isRune}, nil
}

func (r Ref) String() string {
	if r.IsRune {
		return fmt.Sprintf("%s-R%02d", r.SetID, r.CollectorNumber)
	}
	return fmt.Sprintf("%s-%03d%s", r.SetID, r.CollectorNumber, r.Variant)
}
