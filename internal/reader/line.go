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
func (line *Line) HighlightedTokens(plainTextStyle twin.Style, standoutStyle *twin.Style, search *regexp.Regexp, lineIndex *linemetadata.Index) textstyles.StyledRunesWithTrailer {
	plain := line.Plain(lineIndex)
	matchRanges := getMatchRanges(&plain, search)

	var searchHitStyle twin.Style
	if standoutStyle != nil {
		searchHitStyle = *standoutStyle
	} else {
		searchHitStyle = plainTextStyle.WithAttr(twin.AttrReverse)
	}

	var lineHighlightBackground *twin.Color
	if !matchRanges.Empty() {
		// Figure out a line background that lies between plainTextStyle and searchHitStyle
		plainBg := plainTextStyle.Background()
		if plainTextStyle.HasAttr(twin.AttrReverse) {
			plainBg = plainTextStyle.Foreground()
		}
		hitBg := searchHitStyle.Background()
		if searchHitStyle.HasAttr(twin.AttrReverse) {
			hitBg = searchHitStyle.Foreground()
		}

		if plainBg != twin.ColorDefault && hitBg != twin.ColorDefault {
			// We have two real colors. Mix them!
			mixed := plainBg.Mix(hitBg, 0.2)
			lineHighlightBackground = &mixed
		}
	}

	fromString := textstyles.StyledRunesFromString(plainTextStyle, line.raw, lineIndex)
	returnRunes := make([]twin.StyledRune, 0, len(fromString.StyledRunes))
	for _, token := range fromString.StyledRunes {
		style := token.Style
		if matchRanges.InRange(len(returnRunes)) {
			// Highlight the search hit
			style = searchHitStyle
		} else if !matchRanges.Empty() && lineHighlightBackground != nil {
			// Highlight lines that have search hits
			style = style.WithBackground(*lineHighlightBackground)
		}

		returnRunes = append(returnRunes, twin.StyledRune{
			Rune:  token.Rune,
			Style: style,
		})
	}

	trailer := fromString.Trailer
	if !matchRanges.Empty() && lineHighlightBackground != nil {
		// Highlight to the end of the line
		trailer = plainTextStyle.WithBackground(*lineHighlightBackground)
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
