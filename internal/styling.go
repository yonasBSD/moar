package internal

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/chroma/v2"
	log "github.com/sirupsen/logrus"
	"github.com/walles/moor/v2/internal/textstyles"
	"github.com/walles/moor/v2/twin"
)

// From LESS_TERMCAP_so, overrides statusbarStyle from the Chroma style if set
var standoutStyle *twin.Style

var lineNumbersStyle = twin.StyleDefault.WithAttr(twin.AttrDim)

// Status bar and EOF marker style
var statusbarStyle = twin.StyleDefault.WithAttr(twin.AttrReverse)

var plainTextStyle = twin.StyleDefault

var searchHitStyle = twin.StyleDefault.WithAttr(twin.AttrReverse)

// This can be nil
var searchHitLineBackground *twin.Color

func setStyle(updateMe *twin.Style, envVarName string, fallback *twin.Style) {
	envValue := os.Getenv(envVarName)
	if envValue == "" {
		if fallback != nil {
			*updateMe = *fallback
		}
		return
	}

	style, err := TermcapToStyle(envValue)
	if err != nil {
		log.Info("Ignoring invalid ", envVarName, ": ", strings.ReplaceAll(envValue, "\x1b", "ESC"), ": ", err)
		return
	}

	*updateMe = style
}

// With exact set, only return a style if the Chroma formatter has an explicit
// configuration for that style. Otherwise, we might return fallback styles, not
// exactly matching what you requested.
func twinStyleFromChroma(terminalBackground *twin.Color, chromaStyle *chroma.Style, chromaFormatter *chroma.Formatter, chromaToken chroma.TokenType, exact bool) *twin.Style {
	if chromaStyle == nil || chromaFormatter == nil {
		return nil
	}

	stringBuilder := strings.Builder{}
	err := (*chromaFormatter).Format(&stringBuilder, chromaStyle, chroma.Literator(chroma.Token{
		Type:  chromaToken,
		Value: "X",
	}))
	if err != nil {
		panic(err)
	}

	formatted := stringBuilder.String()
	cells := textstyles.StyledRunesFromString(twin.StyleDefault, formatted, nil).StyledRunes
	if len(cells) != 1 {
		log.Warnf("Chroma formatter didn't return exactly one cell: %#v", cells)
		return nil
	}

	inexactStyle := cells[0].Style
	if inexactStyle.Background() == twin.ColorDefault && terminalBackground != nil {
		// Real colors can be mixed & matched, which we do in
		// Line.HighlightedTokens(). So we prefer real colors when we have them.
		inexactStyle = inexactStyle.WithBackground(*terminalBackground)
	}
	if !exact {
		return &inexactStyle
	}

	unstyled := twinStyleFromChroma(terminalBackground, chromaStyle, chromaFormatter, chroma.None, false)
	if unstyled == nil {
		panic("Chroma formatter didn't return a style for chroma.None")
	}
	if inexactStyle != *unstyled {
		// We got something other than the style of None, return it!
		return &inexactStyle
	}

	return nil
}

// consumeLessTermcapEnvs parses LESS_TERMCAP_xx environment variables and
// adapts the moor output accordingly.
func consumeLessTermcapEnvs(terminalBackground *twin.Color, chromaStyle *chroma.Style, chromaFormatter *chroma.Formatter) {
	// Requested here: https://github.com/walles/moor/issues/14

	setStyle(
		&textstyles.ManPageBold,
		"LESS_TERMCAP_md",
		twinStyleFromChroma(terminalBackground, chromaStyle, chromaFormatter, chroma.GenericStrong, false),
	)
	setStyle(&textstyles.ManPageUnderline,
		"LESS_TERMCAP_us",
		twinStyleFromChroma(terminalBackground, chromaStyle, chromaFormatter, chroma.GenericUnderline, false),
	)

	// Since standoutStyle defaults to nil we can't just pass it to setStyle().
	// Instead we give it special treatment here and set it only if its
	// environment variable is set.
	//
	// Ref: https://github.com/walles/moor/issues/171
	envValue := os.Getenv("LESS_TERMCAP_so")
	if envValue != "" {
		style, err := TermcapToStyle(envValue)
		if err == nil {
			log.Trace("Standout style set from LESS_TERMCAP_so: ", style)
			standoutStyle = &style
		} else {
			log.Info("Ignoring invalid LESS_TERMCAP_so: ", strings.ReplaceAll(envValue, "\x1b", "ESC"), ": ", err)
		}
	}
}

func getOppositeColor(base twin.Color) twin.Color {
	if base == twin.ColorDefault {
		panic("can't get opposite of default color")
	}

	white := twin.NewColor24Bit(255, 255, 255)
	black := twin.NewColor24Bit(0, 0, 0)
	if base.Distance(white) > base.Distance(black) {
		// Foreground is far away from white, so pretend the background is white
		return white
	} else {
		// Foreground is far away from black, so pretend the background is black
		return black
	}
}

func styleUI(terminalBackground *twin.Color, chromaStyle *chroma.Style, chromaFormatter *chroma.Formatter, statusbarOption StatusBarOption, withTerminalFg bool) {
	if chromaStyle == nil || chromaFormatter == nil {
		return
	}

	headingStyle := twinStyleFromChroma(terminalBackground, chromaStyle, chromaFormatter, chroma.GenericHeading, true)
	if headingStyle != nil {
		log.Trace("Heading style set from Chroma: ", *headingStyle)
		textstyles.ManPageHeading = *headingStyle
	}

	chromaLineNumbers := twinStyleFromChroma(terminalBackground, chromaStyle, chromaFormatter, chroma.LineNumbers, true)
	if chromaLineNumbers != nil {
		// NOTE: We used to dim line numbers here, but Johan found them too hard
		// to read. If line numbers should look some other way for some Chroma
		// style, go fix that in Chroma!
		log.Trace("Line numbers style set from Chroma: ", *chromaLineNumbers)
		lineNumbersStyle = *chromaLineNumbers
	}

	if withTerminalFg {
		plainTextStyle = twin.StyleDefault
		lineNumbersStyle = twin.StyleDefault.WithAttr(twin.AttrDim)
	} else {
		plainText := twinStyleFromChroma(terminalBackground, chromaStyle, chromaFormatter, chroma.None, false)
		if plainText != nil {
			log.Trace("Plain text style set from Chroma: ", *plainText)
			plainTextStyle = *plainText
		}
	}

	if standoutStyle != nil {
		log.Trace("Status bar style set from standout style: ", *standoutStyle)
		statusbarStyle = *standoutStyle
	} else if statusbarOption == STATUSBAR_STYLE_INVERSE {
		statusbarStyle = plainTextStyle.WithAttr(twin.AttrReverse)
	} else if statusbarOption == STATUSBAR_STYLE_PLAIN {
		plain := twinStyleFromChroma(terminalBackground, chromaStyle, chromaFormatter, chroma.None, false)
		if plain != nil {
			statusbarStyle = *plain
		} else {
			statusbarStyle = twin.StyleDefault
		}
	} else if statusbarOption == STATUSBAR_STYLE_BOLD {
		bold := twinStyleFromChroma(terminalBackground, chromaStyle, chromaFormatter, chroma.GenericStrong, true)
		if bold != nil {
			statusbarStyle = *bold
		} else {
			statusbarStyle = twin.StyleDefault.WithAttr(twin.AttrBold)
		}
	} else {
		panic(fmt.Sprint("Unrecognized status bar style: ", statusbarOption))
	}

	configureHighlighting(terminalBackground)
}

// Expects to be called from the end of styleUI(), since at that
// point we should have all data we need to set up highlighting.
func configureHighlighting(terminalBackground *twin.Color) {
	if standoutStyle != nil {
		searchHitStyle = *standoutStyle
		log.Trace("Search hit style set from standout style: ", searchHitStyle)
	} else {
		log.Trace("Search hit style set to default: ", searchHitStyle)
	}

	// Figure out a line background that lies between plainTextStyle and searchHit
	var plainBg twin.Color
	if terminalBackground != nil {
		plainBg = *terminalBackground
	} else if plainTextStyle.HasAttr(twin.AttrReverse) {
		plainBg = plainTextStyle.Foreground()
	} else {
		plainBg = plainTextStyle.Background()
	}

	hitBg := searchHitStyle.Background()
	hitFg := searchHitStyle.Foreground()
	if searchHitStyle.HasAttr(twin.AttrReverse) {
		hitBg = searchHitStyle.Foreground()
		hitFg = searchHitStyle.Background()
	}
	if hitBg == twin.ColorDefault && hitFg != twin.ColorDefault {
		// Not knowing the hit background color will be a problem further down
		// when we want to create a line background color for lines with search
		// hits.
		//
		// But since we know the foreground color, we can cheat and pretend the
		// background is as far away from the foreground as possible.
		hitBg = getOppositeColor(hitFg)
	}
	if hitBg == twin.ColorDefault && terminalBackground != nil {
		// Assume the hit background is the opposite of the terminal background
		hitBg = getOppositeColor(*terminalBackground)
	}

	if plainBg != twin.ColorDefault && hitBg != twin.ColorDefault {
		// We have two real colors. Mix them! I got to "0.2" by testing some
		// numbers. 0.2 is visible but not too strong.
		mixed := plainBg.Mix(hitBg, 0.2)
		searchHitLineBackground = &mixed

		log.Trace("Search hit line background set to mixed color: ", *searchHitLineBackground)
	} else {
		log.Debug("Cannot set search hit line background based on plainBg=", plainBg, " hitBg=", hitBg)
	}
}

func TermcapToStyle(termcap string) (twin.Style, error) {
	// Add a character to be sure we have one to take the format from
	cells := textstyles.StyledRunesFromString(twin.StyleDefault, termcap+"x", nil).StyledRunes
	if len(cells) != 1 {
		return twin.StyleDefault, fmt.Errorf("Expected styling only and no text")
	}
	return cells[len(cells)-1].Style, nil
}
