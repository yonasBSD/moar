// This package handles styled strings. It can strip styling from strings and it
// can turn a styled string into a series of screen cells. Some global variables
// can be used to configure how various things are rendered.
package textstyles

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/twin"
)

// How do we render unprintable characters?
type UnprintableStyleT int

const (
	UnprintableStyleHighlight UnprintableStyleT = iota
	UnprintableStyleWhitespace
)

var UnprintableStyle UnprintableStyleT

// These three styles will be configured from styling.go
var ManPageBold = twin.StyleDefault.WithAttr(twin.AttrBold)
var ManPageUnderline = twin.StyleDefault.WithAttr(twin.AttrUnderline)
var ManPageHeading = twin.StyleDefault.WithAttr(twin.AttrBold)

// This is what less (version 581.2 on macOS) defaults to
var TabSize = 8

const BACKSPACE = '\b'

type StyledRunesWithTrailer struct {
	StyledRunes []CellWithMetadata
	Trailer     twin.Style
}

func isPlain(s string) bool {
	for i := 0; i < len(s); i++ {
		byteAtIndex := s[i]
		if byteAtIndex < 32 {
			return false
		}
		if byteAtIndex > 127 {
			return false
		}
	}

	return true
}

func WithoutFormatting(s string, lineIndex *linemetadata.Index) string {
	if isPlain(s) {
		return s
	}

	stripped := strings.Builder{}
	runeCount := 0

	// " * 2" here makes BenchmarkPlainTextSearch() perform 30% faster. Probably
	// due to avoiding a number of additional implicit Grow() calls when adding
	// runes.
	stripped.Grow(len(s) * 2)

	styledStringsFromString(twin.StyleDefault, s, lineIndex, func(str string, style twin.Style) {
		for _, runeValue := range runesFromStyledString(_StyledString{String: str, Style: style}) {
			switch runeValue {

			case '\x09': // TAB
				for {
					stripped.WriteRune(' ')
					runeCount++

					if runeCount%TabSize == 0 {
						// We arrived at the next tab stop
						break
					}
				}

			case '�': // Go's broken-UTF8 marker
				switch UnprintableStyle {
				case UnprintableStyleHighlight:
					stripped.WriteRune('?')
				case UnprintableStyleWhitespace:
					stripped.WriteRune(' ')
				default:
					panic(fmt.Errorf("Unsupported unprintable-style: %#v", UnprintableStyle))
				}
				runeCount++

			case BACKSPACE:
				stripped.WriteRune('<')
				runeCount++

			default:
				if !twin.Printable(runeValue) {
					stripped.WriteRune('?')
					runeCount++
					continue
				}
				stripped.WriteRune(runeValue)
				runeCount++
			}
		}
	})

	return stripped.String()
}

// Turn a (formatted) string into a series of screen cells
//
// The prefix will be prepended to the string before parsing. The lineIndex is
// used for error reporting.
func StyledRunesFromString(plainTextStyle twin.Style, s string, lineIndex *linemetadata.Index) StyledRunesWithTrailer {
	manPageHeading := manPageHeadingFromString(s)
	if manPageHeading != nil {
		return *manPageHeading
	}

	cells := make([]CellWithMetadata, 0, len(s))

	// Specs: https://en.wikipedia.org/wiki/ANSI_escape_code#3-bit_and_4-bit
	styleUnprintable := twin.StyleDefault.WithBackground(twin.NewColor16(1)).WithForeground(twin.NewColor16(7))

	trailer := styledStringsFromString(plainTextStyle, s, lineIndex, func(str string, style twin.Style) {
		for _, token := range tokensFromStyledString(_StyledString{String: str, Style: style}) {
			switch token.Rune {

			case '\x09': // TAB
				for {
					cells = append(cells, CellWithMetadata{
						Rune:  ' ',
						Style: style,
					})

					if (len(cells))%TabSize == 0 {
						// We arrived at the next tab stop
						break
					}
				}

			case '�': // Go's broken-UTF8 marker
				switch UnprintableStyle {
				case UnprintableStyleHighlight:
					cells = append(cells, CellWithMetadata{
						Rune:  '?',
						Style: styleUnprintable,
					})
				case UnprintableStyleWhitespace:
					cells = append(cells, CellWithMetadata{
						Rune:  '?',
						Style: twin.StyleDefault,
					})
				default:
					panic(fmt.Errorf("Unsupported unprintable-style: %#v", UnprintableStyle))
				}

			case BACKSPACE:
				cells = append(cells, CellWithMetadata{
					Rune:  '<',
					Style: styleUnprintable,
				})

			default:
				if !twin.Printable(token.Rune) {
					switch UnprintableStyle {
					case UnprintableStyleHighlight:
						cells = append(cells, CellWithMetadata{
							Rune:  '?',
							Style: styleUnprintable,
						})
					case UnprintableStyleWhitespace:
						cells = append(cells, CellWithMetadata{
							Rune:  ' ',
							Style: twin.StyleDefault,
						})
					default:
						panic(fmt.Errorf("Unsupported unprintable-style: %#v", UnprintableStyle))
					}
					continue
				}
				cells = append(cells, CellWithMetadata{
					Rune:  token.Rune,
					Style: token.Style,
				})
			}
		}
	})

	return StyledRunesWithTrailer{
		StyledRunes: cells,
		Trailer:     trailer,
	}
}

// Consume '_<x<x', where '<' is backspace and the result is a bold underlined 'x'
func consumeBoldUnderline(runes []rune, index int) (int, *twin.StyledRune) {
	if index+4 >= len(runes) {
		// Not enough runes left for a bold underline
		return index, nil
	}

	if runes[index] != '_' {
		// No initial underscore
		return index, nil
	}

	if runes[index+1] != BACKSPACE {
		// No first backspace
		return index, nil
	}

	if runes[index+2] != runes[index+4] {
		// Runes don't match
		return index, nil
	}

	if runes[index+3] != BACKSPACE {
		// No second backspace
		return index, nil
	}

	// Merge ManPageUnderline attributes into ManPageBold to form boldUnderline.
	// Based on the screenshots here: https://github.com/walles/moor/issues/310
	boldUnderline := ManPageBold
	if ManPageUnderline.HasAttr(twin.AttrUnderline) {
		boldUnderline = boldUnderline.WithAttr(twin.AttrUnderline)
	}
	if ManPageUnderline.HasAttr(twin.AttrItalic) {
		boldUnderline = boldUnderline.WithAttr(twin.AttrItalic)
	}

	// We have a match!
	return index + 5, &twin.StyledRune{
		Rune:  runes[index+4],
		Style: boldUnderline,
	}
}

// Consume 'x<x', where '<' is backspace and the result is a bold 'x'
func consumeBold(runes []rune, index int) (int, *twin.StyledRune) {
	if index+2 >= len(runes) {
		// Not enough runes left for a bold
		return index, nil
	}

	if runes[index+1] != BACKSPACE {
		// No backspace in the middle, never mind
		return index, nil
	}

	if runes[index] != runes[index+2] {
		// First and last rune not the same, never mind
		return index, nil
	}

	// We have a match!
	return index + 3, &twin.StyledRune{
		Rune:  runes[index],
		Style: ManPageBold,
	}
}

// Consume '_<x', where '<' is backspace and the result is an underlined 'x'
func consumeUnderline(runes []rune, index int) (int, *twin.StyledRune) {
	if index+2 >= len(runes) {
		// Not enough runes left for a underline
		return index, nil
	}

	if runes[index+1] != BACKSPACE {
		// No backspace in the middle, never mind
		return index, nil
	}

	if runes[index] != '_' {
		// No underline, never mind
		return index, nil
	}

	// We have a match!
	return index + 3, &twin.StyledRune{
		Rune:  runes[index+2],
		Style: ManPageUnderline,
	}
}

// Consume '+<+<o<o' / '+<o', where '<' is backspace and the result is a unicode bullet.
//
// Used on man pages, try "man printf" on macOS for one example.
func consumeBullet(runes []rune, index int) (int, *twin.StyledRune) {
	patterns := [][]byte{[]byte("+\bo"), []byte("+\b+\bo\bo")}
	for _, pattern := range patterns {
		if index+len(pattern) > len(runes) {
			// Not enough runes left for a bullet
			continue
		}

		mismatch := false
		for delta, patternByte := range pattern {
			if patternByte != byte(runes[index+delta]) {
				// Bullet pattern mismatch, never mind
				mismatch = true
				break
			}
		}
		if mismatch {
			continue
		}

		// We have a match!
		return index + len(pattern), &twin.StyledRune{
			Rune:  '•', // Unicode bullet point
			Style: twin.StyleDefault,
		}
	}

	return index, nil
}

func runesFromStyledString(styledString _StyledString) string {
	hasBackspace := slices.Contains([]byte(styledString.String), BACKSPACE)

	if !hasBackspace {
		// Shortcut when there's no backspace based formatting to worry about
		return styledString.String
	}

	// Special handling for man page formatted lines
	cells := tokensFromStyledString(styledString)
	returnMe := strings.Builder{}
	returnMe.Grow(len(cells))
	for _, cell := range cells {
		returnMe.WriteRune(cell.Rune)
	}

	return returnMe.String()
}

func tokensFromStyledString(styledString _StyledString) []twin.StyledRune {
	runes := []rune(styledString.String)

	hasBackspace := false
	for _, runeValue := range runes {
		if runeValue == BACKSPACE {
			hasBackspace = true
			break
		}
	}

	tokens := make([]twin.StyledRune, 0, len(runes))
	if !hasBackspace {
		// Shortcut when there's no backspace based formatting to worry about
		for _, runeValue := range runes {
			tokens = append(tokens, twin.StyledRune{
				Rune:  runeValue,
				Style: styledString.Style,
			})
		}
		return tokens
	}

	// Special handling for man page formatted lines
	for index := 0; index < len(runes); index++ {
		nextIndex, token := consumeBullet(runes, index)
		if nextIndex != index {
			tokens = append(tokens, *token)
			index = nextIndex - 1
			continue
		}

		nextIndex, token = consumeBoldUnderline(runes, index)
		if nextIndex != index {
			tokens = append(tokens, *token)
			index = nextIndex - 1
			continue
		}

		nextIndex, token = consumeBold(runes, index)
		if nextIndex != index {
			tokens = append(tokens, *token)
			index = nextIndex - 1
			continue
		}

		nextIndex, token = consumeUnderline(runes, index)
		if nextIndex != index {
			tokens = append(tokens, *token)
			index = nextIndex - 1
			continue
		}

		tokens = append(tokens, twin.StyledRune{
			Rune:  runes[index],
			Style: styledString.Style,
		})
	}

	return tokens
}

type _StyledString struct {
	String string
	Style  twin.Style
}

// To avoid allocations, our caller is expected to provide us with a
// pre-allocated numbersBuffer for storing the result.
//
// This function is part of the hot code path while searching, so we want it to
// be fast.
//
// # Benchmarking instructions
//
//	go test -benchmem -run='^$' -bench=BenchmarkHighlightedSearch . ./...
func splitIntoNumbers(s string, numbersBuffer []uint) ([]uint, error) {
	numbers := numbersBuffer[:0]

	afterLastSeparator := 0
	for i, char := range s {
		if char >= '0' && char <= '9' {
			continue
		}

		if char == ';' || char == ':' {
			numberString := s[afterLastSeparator:i]
			if numberString == "" {
				numbers = append(numbers, 0)
				continue
			}

			number, err := strconv.ParseUint(numberString, 10, 64)
			if err != nil {
				return numbers, err
			}
			numbers = append(numbers, uint(number))
			afterLastSeparator = i + 1
			continue
		}

		return numbers, fmt.Errorf("Unrecognized character in <%s>: %c", s, char)
	}

	// Now we have to handle the last number
	numberString := s[afterLastSeparator:]
	if numberString == "" {
		numbers = append(numbers, 0)
		return numbers, nil
	}
	number, err := strconv.ParseUint(numberString, 10, 64)
	if err != nil {
		return numbers, err
	}
	numbers = append(numbers, uint(number))

	return numbers, nil
}

// rawUpdateStyle parses a string of the form "33m" into changes to style. This
// is what comes after ESC[ in an ANSI SGR sequence.
func rawUpdateStyle(style twin.Style, escapeSequenceWithoutHeader string, numbersBuffer []uint) (twin.Style, []uint, error) {
	if len(escapeSequenceWithoutHeader) == 0 {
		return style, numbersBuffer, fmt.Errorf("empty escape sequence, expected at least an ending letter")
	}
	if escapeSequenceWithoutHeader[len(escapeSequenceWithoutHeader)-1] != 'm' {
		return style, numbersBuffer, fmt.Errorf("escape sequence does not end with 'm': %s", escapeSequenceWithoutHeader)
	}

	numbersBuffer, err := splitIntoNumbers(escapeSequenceWithoutHeader[:len(escapeSequenceWithoutHeader)-1], numbersBuffer)
	if err != nil {
		return style, numbersBuffer, fmt.Errorf("splitIntoNumbers: %w", err)
	}

	index := 0
	for index < len(numbersBuffer) {
		number := numbersBuffer[index]
		index++
		switch number {
		case 0:
			// SGR Reset should not affect the OSC8 hyperlink
			style = twin.StyleDefault.WithHyperlink(style.HyperlinkURL())

		case 1:
			style = style.WithAttr(twin.AttrBold)

		case 2:
			style = style.WithAttr(twin.AttrDim)

		case 3:
			style = style.WithAttr(twin.AttrItalic)

		case 4:
			style = style.WithAttr(twin.AttrUnderline)

		case 7:
			style = style.WithAttr(twin.AttrReverse)

		case 22:
			style = style.WithoutAttr(twin.AttrBold).WithoutAttr(twin.AttrDim)

		case 23:
			style = style.WithoutAttr(twin.AttrItalic)

		case 24:
			style = style.WithoutAttr(twin.AttrUnderline)

		case 27:
			style = style.WithoutAttr(twin.AttrReverse)

		// Foreground colors, https://pkg.go.dev/github.com/gdamore/tcell#Color
		case 30:
			style = style.WithForeground(twin.NewColor16(0))
		case 31:
			style = style.WithForeground(twin.NewColor16(1))
		case 32:
			style = style.WithForeground(twin.NewColor16(2))
		case 33:
			style = style.WithForeground(twin.NewColor16(3))
		case 34:
			style = style.WithForeground(twin.NewColor16(4))
		case 35:
			style = style.WithForeground(twin.NewColor16(5))
		case 36:
			style = style.WithForeground(twin.NewColor16(6))
		case 37:
			style = style.WithForeground(twin.NewColor16(7))
		case 38:
			var err error
			var color *twin.Color
			index, color, err = consumeCompositeColor(numbersBuffer, index-1)
			if err != nil {
				return style, numbersBuffer, fmt.Errorf("Foreground: %w", err)
			}
			style = style.WithForeground(*color)
		case 39:
			style = style.WithForeground(twin.ColorDefault)

		// Background colors, see https://pkg.go.dev/github.com/gdamore/Color
		case 40:
			style = style.WithBackground(twin.NewColor16(0))
		case 41:
			style = style.WithBackground(twin.NewColor16(1))
		case 42:
			style = style.WithBackground(twin.NewColor16(2))
		case 43:
			style = style.WithBackground(twin.NewColor16(3))
		case 44:
			style = style.WithBackground(twin.NewColor16(4))
		case 45:
			style = style.WithBackground(twin.NewColor16(5))
		case 46:
			style = style.WithBackground(twin.NewColor16(6))
		case 47:
			style = style.WithBackground(twin.NewColor16(7))
		case 48:
			var err error
			var color *twin.Color
			index, color, err = consumeCompositeColor(numbersBuffer, index-1)
			if err != nil {
				return style, numbersBuffer, fmt.Errorf("Background: %w", err)
			}
			style = style.WithBackground(*color)
		case 49:
			style = style.WithBackground(twin.ColorDefault)

		case 58:
			var err error
			var color *twin.Color
			index, color, err = consumeCompositeColor(numbersBuffer, index-1)
			if err != nil {
				return style, numbersBuffer, fmt.Errorf("Underline: %w", err)
			}
			style = style.WithUnderlineColor(*color)

		case 59:
			style = style.WithUnderlineColor(twin.ColorDefault)

		// Bright foreground colors: see https://pkg.go.dev/github.com/gdamore/Color
		//
		// After testing vs less and cat on iTerm2 3.3.9 / macOS Catalina
		// 10.15.4 that's how they seem to handle this, tested with:
		// * TERM=xterm-256color
		// * TERM=screen-256color
		case 90:
			style = style.WithForeground(twin.NewColor16(8))
		case 91:
			style = style.WithForeground(twin.NewColor16(9))
		case 92:
			style = style.WithForeground(twin.NewColor16(10))
		case 93:
			style = style.WithForeground(twin.NewColor16(11))
		case 94:
			style = style.WithForeground(twin.NewColor16(12))
		case 95:
			style = style.WithForeground(twin.NewColor16(13))
		case 96:
			style = style.WithForeground(twin.NewColor16(14))
		case 97:
			style = style.WithForeground(twin.NewColor16(15))

		case 100:
			style = style.WithBackground(twin.NewColor16(8))
		case 101:
			style = style.WithBackground(twin.NewColor16(9))
		case 102:
			style = style.WithBackground(twin.NewColor16(10))
		case 103:
			style = style.WithBackground(twin.NewColor16(11))
		case 104:
			style = style.WithBackground(twin.NewColor16(12))
		case 105:
			style = style.WithBackground(twin.NewColor16(13))
		case 106:
			style = style.WithBackground(twin.NewColor16(14))
		case 107:
			style = style.WithBackground(twin.NewColor16(15))

		default:
			return style, numbersBuffer, fmt.Errorf("Unrecognized ANSI SGR code <%d>", number)
		}
	}

	return style, numbersBuffer, nil
}

func joinUints(ints []uint) string {
	joinedWithBrackets := strings.ReplaceAll(fmt.Sprint(ints), " ", ";")
	joined := joinedWithBrackets[1 : len(joinedWithBrackets)-1]
	return joined
}

// numbers is a list of numbers from a ANSI SGR string
// index points to either 38 or 48 in that string
//
// This method will return:
//   - The first index in the string that this function did not consume
//   - A color value that can be applied to a style
func consumeCompositeColor(numbers []uint, index int) (int, *twin.Color, error) {
	baseIndex := index
	if numbers[index] != 38 && numbers[index] != 48 && numbers[index] != 58 {
		err := fmt.Errorf(
			"unknown start of color sequence <%d>, expected 38 (foreground), 48 (background) or 58 (underline): <CSI %sm>",
			numbers[index],
			joinUints(numbers[baseIndex:]))
		return -1, nil, err
	}

	index++
	if index >= len(numbers) {
		err := fmt.Errorf(
			"incomplete color sequence: <CSI %sm>",
			joinUints(numbers[baseIndex:]))
		return -1, nil, err
	}

	if numbers[index] == 5 {
		// Handle 8 bit color
		index++
		if index >= len(numbers) {
			err := fmt.Errorf(
				"incomplete 8 bit color sequence: <CSI %sm>",
				joinUints(numbers[baseIndex:]))
			return -1, nil, err
		}

		colorNumber := numbers[index]

		colorValue := twin.NewColor256(uint8(colorNumber))
		return index + 1, &colorValue, nil
	}

	if numbers[index] == 2 {
		// Handle 24 bit color
		rIndex := index + 1
		gIndex := index + 2
		bIndex := index + 3
		if bIndex >= len(numbers) {
			err := fmt.Errorf(
				"incomplete 24 bit color sequence, expected N8;2;R;G;Bm: <CSI %sm>",
				joinUints(numbers[baseIndex:]))

			return -1, nil, err
		}

		rValue := uint8(numbers[rIndex])
		gValue := uint8(numbers[gIndex])
		bValue := uint8(numbers[bIndex])

		colorValue := twin.NewColor24Bit(rValue, gValue, bValue)

		return bIndex + 1, &colorValue, nil
	}

	err := fmt.Errorf(
		"unknown color type <%d>, expected 5 (8 bit color) or 2 (24 bit color): <CSI %sm>",
		numbers[index],
		joinUints(numbers[baseIndex:]))

	return -1, nil, err
}
