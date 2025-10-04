package reader

import (
	"regexp"
	"sync"

	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/internal/textstyles"
	"github.com/walles/moor/v2/twin"
)

// A Line represents a line of text that can / will be paged
type Line struct {
	raw   string
	plain *string
	lock  sync.Mutex
}

// NewLine creates a new Line from a (potentially ANSI / man page formatted) string
func NewLine(raw string) Line {
	return Line{
		raw:   raw,
		plain: nil,
		lock:  sync.Mutex{},
	}
}

// Returns a representation of the string split into styled tokens. Any regexp
// matches are highlighted. A nil regexp means no highlighting.
func (line *Line) HighlightedTokens(
	plainTextStyle twin.Style,
	searchHitStyle twin.Style,
	searchHitLineBackground *twin.Color,
	search *regexp.Regexp,
	lineIndex *linemetadata.Index,
) textstyles.StyledRunesWithTrailer {
	plain := line.Plain(lineIndex)
	matchRanges := getMatchRanges(&plain, search)

	fromString := textstyles.StyledRunesFromString(plainTextStyle, line.raw, lineIndex)
	returnRunes := make([]textstyles.CellWithMetadata, 0, len(fromString.StyledRunes))
	lastWasSearchHit := false
	for _, token := range fromString.StyledRunes {
		style := token.Style
		searchHit := matchRanges.InRange(len(returnRunes))
		if searchHit {
			// Highlight the search hit
			style = searchHitStyle
		} else if !matchRanges.Empty() && searchHitLineBackground != nil {
			// Highlight lines that have search hits
			style = style.WithBackground(*searchHitLineBackground)
		}

		returnRunes = append(returnRunes, textstyles.CellWithMetadata{
			Rune:            token.Rune,
			Style:           style,
			StartsSearchHit: searchHit && !lastWasSearchHit,
		})
		lastWasSearchHit = searchHit
	}

	trailer := fromString.Trailer
	if !matchRanges.Empty() && searchHitLineBackground != nil {
		// Highlight to the end of the line
		trailer = plainTextStyle.WithBackground(*searchHitLineBackground)
	}

	return textstyles.StyledRunesWithTrailer{
		StyledRunes: returnRunes,
		Trailer:     trailer,
	}
}

// Plain returns a plain text representation of the initial string
func (line *Line) Plain(lineIndex *linemetadata.Index) string {
	line.lock.Lock()
	defer line.lock.Unlock()

	if line.plain == nil {
		plain := textstyles.WithoutFormatting(line.raw, lineIndex)
		line.plain = &plain
	}
	return *line.plain
}
