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

func getHighlights(plainTextStyle twin.Style, standoutStyle *twin.Style) (searchHit twin.Style, lineHitBackground *twin.Color) {
	if standoutStyle != nil {
		searchHit = *standoutStyle
	} else {
		searchHit = plainTextStyle.WithAttr(twin.AttrReverse)
	}

	// Figure out a line background that lies between plainTextStyle and searchHit
	plainBg := plainTextStyle.Background()
	if plainTextStyle.HasAttr(twin.AttrReverse) {
		plainBg = plainTextStyle.Foreground()
	}
	hitBg := searchHit.Background()
	hitFg := searchHit.Foreground()
	if searchHit.HasAttr(twin.AttrReverse) {
		hitBg = searchHit.Foreground()
		hitFg = searchHit.Background()
	}
	if hitBg == twin.ColorDefault && hitFg != twin.ColorDefault {
		// Not knowing the hit background color will be a problem further down
		// when we want to create a line background color.
		//
		// But since we know the foreground color, we can cheat and pretend the
		// background is as far away from the foreground as possible.
		white := twin.NewColor24Bit(255, 255, 255)
		black := twin.NewColor24Bit(0, 0, 0)
		if hitFg.Distance(white) > hitFg.Distance(black) {
			// Foreground is far away from white, so pretend the background is white
			hitBg = white
		} else {
			// Foreground is far away from black, so pretend the background is black
			hitBg = black
		}
	}

	if plainBg != twin.ColorDefault && hitBg != twin.ColorDefault {
		// We have two real colors. Mix them! I got to "0.2" by testing some
		// numbers. 0.2 is visible but not too strong.
		mixed := plainBg.Mix(hitBg, 0.2)
		lineHitBackground = &mixed
	}

	return // searchHit, lineHitBackground
}

// Returns a representation of the string split into styled tokens. Any regexp
// matches are highlighted. A nil regexp means no highlighting.
func (line *Line) HighlightedTokens(plainTextStyle twin.Style, standoutStyle *twin.Style, search *regexp.Regexp, lineIndex *linemetadata.Index) textstyles.StyledRunesWithTrailer {
	plain := line.Plain(lineIndex)
	matchRanges := getMatchRanges(&plain, search)

	var searchHitStyle twin.Style
	var lineHighlightBackground *twin.Color
	if !matchRanges.Empty() {
		searchHitStyle, lineHighlightBackground = getHighlights(plainTextStyle, standoutStyle)
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
