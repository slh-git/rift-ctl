package cards

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// FormatPretty writes a human-readable card summary to w.
func FormatPretty(w io.Writer, c Card) error {
	ref := Ref{
		SetID:           c.SetID,
		CollectorNumber: c.CollectorNumber,
		Variant:         c.Variant,
		IsRune:          strings.HasPrefix(strings.ToLower(c.ID), strings.ToLower(c.SetID)+"-r"),
	}
	header := strings.TrimSpace(fmt.Sprintf("%s  %s  (%s)", c.Name, ref.String(), c.ID))
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}

	metaParts := make([]string, 0, 4)
	if c.Supertype != "" {
		metaParts = append(metaParts, c.Supertype)
	}
	if c.Type != "" {
		metaParts = append(metaParts, c.Type)
	}
	if len(c.Domains) > 0 {
		metaParts = append(metaParts, strings.Join(c.Domains, ", "))
	}
	if c.Rarity != "" {
		metaParts = append(metaParts, c.Rarity)
	}
	if len(metaParts) > 0 {
		if _, err := fmt.Fprintln(w, strings.Join(metaParts, " · ")); err != nil {
			return err
		}
	}

	statParts := make([]string, 0, 3)
	if c.Energy != nil {
		statParts = append(statParts, "Energy "+strconv.Itoa(*c.Energy))
	}
	if c.Might != nil {
		statParts = append(statParts, "Might "+strconv.Itoa(*c.Might))
	}
	if c.Power != nil {
		statParts = append(statParts, "Power "+strconv.Itoa(*c.Power))
	}
	if len(statParts) > 0 {
		if _, err := fmt.Fprintln(w, strings.Join(statParts, "  ")); err != nil {
			return err
		}
	}

	if text := strings.TrimSpace(c.TextPlain); text != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, text); err != nil {
			return err
		}
	}
	if flavour := strings.TrimSpace(c.Flavour); flavour != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%q\n", flavour); err != nil {
			return err
		}
	}
	if artist := strings.TrimSpace(c.Artist); artist != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Artist: %s\n", artist); err != nil {
			return err
		}
	}
	return nil
}
