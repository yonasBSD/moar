package reader

import (
	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/internal/search"
	"github.com/walles/moor/v2/internal/textstyles"
	"github.com/walles/moor/v2/twin"
)

// Returns a representation of the string split into styled tokens. Any regexp
// matches are highlighted. A nil regexp means no highlighting.
//
// minRunesCount: at least this many runes will be included in the result. If 0,
// do all runes. For BenchmarkRenderHugeLine() performance.
func (line *Line) HighlightedTokens(
	plainTextStyle twin.Style,
	searchHitStyle twin.Style,
	search search.Search,
	lineIndex linemetadata.Index,
	minRunesCount int,
) textstyles.StyledRunesWithTrailer {
	matchRanges := search.GetMatchRanges(line.Plain(lineIndex))

	fromString := textstyles.StyledRunesFromString(plainTextStyle, string(line.raw), &lineIndex, minRunesCount)
	returnRunes := make([]textstyles.CellWithMetadata, 0, len(fromString.StyledRunes))
	lastWasSearchHit := false
	for _, token := range fromString.StyledRunes {
		style := token.Style
		searchHit := matchRanges.InRange(len(returnRunes))
		if searchHit {
			// Highlight the search hit
			style = searchHitStyle
		}

		returnRunes = append(returnRunes, textstyles.CellWithMetadata{
			Rune:            token.Rune,
			Style:           style,
			IsSearchHit:     searchHit,
			StartsSearchHit: searchHit && !lastWasSearchHit,
		})
		lastWasSearchHit = searchHit
	}

	return textstyles.StyledRunesWithTrailer{
		StyledRunes:       returnRunes,
		Trailer:           fromString.Trailer,
		ContainsSearchHit: !matchRanges.Empty(),
	}
}

func (line *Line) HasManPageFormatting() bool {
	return textstyles.HasManPageFormatting(string(line.raw))
}
