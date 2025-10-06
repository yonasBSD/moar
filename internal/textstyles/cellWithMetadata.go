package textstyles

import (
	"unicode"

	"github.com/walles/moor/v2/twin"
)

// Like twin.StyledRune, but with additional metadata
type CellWithMetadata struct {
	Rune  rune
	Style twin.Style

	StartsSearchHit bool // True if this cell is the first cell of a search hit
}

func (r CellWithMetadata) ToStyledRune() twin.StyledRune {
	return twin.NewStyledRune(r.Rune, r.Style)
}

func (r CellWithMetadata) Width() int {
	return r.ToStyledRune().Width()
}

type CellWithMetadataSlice []CellWithMetadata

// Returns a copy of the slice with leading whitespace removed
func (runes CellWithMetadataSlice) WithoutSpaceLeft() CellWithMetadataSlice {
	for i := range runes {
		cell := runes[i]
		if !unicode.IsSpace(cell.Rune) {
			return runes[i:]
		}

		// That was a space, keep looking
	}

	// All whitespace, return empty
	return CellWithMetadataSlice{}
}

// Returns a copy of the slice with trailing whitespace removed
func (runes CellWithMetadataSlice) WithoutSpaceRight() CellWithMetadataSlice {
	for i := len(runes) - 1; i >= 0; i-- {
		cell := runes[i]
		if !unicode.IsSpace(cell.Rune) {
			return runes[0 : i+1]
		}

		// That was a space, keep looking
	}

	// All whitespace, return empty
	return CellWithMetadataSlice{}
}
