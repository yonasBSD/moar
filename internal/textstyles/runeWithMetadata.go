package textstyles

import "github.com/walles/moor/v2/twin"

// Like twin.StyledRune, but with additional metadata
type RuneWithMetadata struct {
	Rune         rune
	Style        twin.Style
	HasSearchHit bool // True if this rune is part of a search hit
}

func (r RuneWithMetadata) ToStyledRune() twin.StyledRune {
	return twin.NewStyledRune(r.Rune, r.Style)
}
