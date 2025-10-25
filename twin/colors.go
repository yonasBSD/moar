package twin

import (
	"fmt"
	"math"

	"github.com/alecthomas/chroma/v2"
)

// Create using NewColor16(), NewColor256 or NewColor24Bit(), or use
// ColorDefault.
type Color uint32

type ColorCount uint8

const (
	// Default foreground / background color
	ColorCountDefault ColorCount = iota

	// https://en.wikipedia.org/wiki/ANSI_escape_code#3-bit_and_4-bit
	//
	// Note that this type is only used for output, on input we store 3 bit
	// colors as 4 bit colors since they map to the same values.
	ColorCount8

	// https://en.wikipedia.org/wiki/ANSI_escape_code#3-bit_and_4-bit
	ColorCount16

	// https://en.wikipedia.org/wiki/ANSI_escape_code#8-bit
	ColorCount256

	// RGB: https://en.wikipedia.org/wiki/ANSI_escape_code#24-bit
	ColorCount24bit
)

type colorType uint8

const (
	colorTypeForeground colorType = iota
	colorTypeBackground
	colorTypeUnderline
)

// Reset to default foreground / background color
var ColorDefault = newColor(ColorCountDefault, 0)

// From: https://en.wikipedia.org/wiki/ANSI_escape_code#3-bit_and_4-bit
var colorNames16 = map[int]string{
	0:  "0 black",
	1:  "1 red",
	2:  "2 green",
	3:  "3 yellow (orange)",
	4:  "4 blue",
	5:  "5 magenta",
	6:  "6 cyan",
	7:  "7 white (light gray)",
	8:  "8 bright black (dark gray)",
	9:  "9 bright red",
	10: "10 bright green",
	11: "11 bright yellow",
	12: "12 bright blue",
	13: "13 bright magenta",
	14: "14 bright cyan",
	15: "15 bright white",
}

func newColor(colorCount ColorCount, value uint32) Color {
	return Color(value | (uint32(colorCount) << 24))
}

// Four bit colors as defined here:
// https://en.wikipedia.org/wiki/ANSI_escape_code#3-bit_and_4-bit
func NewColor16(colorNumber0to15 int) Color {
	return newColor(ColorCount16, uint32(colorNumber0to15))
}

func NewColor256(colorNumber uint8) Color {
	return newColor(ColorCount256, uint32(colorNumber))
}

func NewColor24Bit(red uint8, green uint8, blue uint8) Color {
	return newColor(ColorCount24bit, (uint32(red)<<16)+(uint32(green)<<8)+(uint32(blue)<<0))
}

func NewColorHex(rgb uint32) Color {
	return newColor(ColorCount24bit, rgb)
}

func (color Color) ColorCount() ColorCount {
	return ColorCount(color >> 24)
}

func (color Color) colorValue() uint32 {
	return uint32(color & 0xff_ff_ff)
}

// Render color into an ANSI string.
//
// Ref: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
func (color Color) ansiString(cType colorType, terminalColorCount ColorCount) string {
	var typeMarker string
	if cType == colorTypeForeground {
		typeMarker = "3"
	} else if cType == colorTypeBackground {
		typeMarker = "4"
	} else if cType == colorTypeUnderline {
		typeMarker = "5"
	} else {
		panic(fmt.Errorf("unhandled color type %d", cType))
	}

	if color.ColorCount() == ColorCountDefault {
		return fmt.Sprint("\x1b[", typeMarker, "9m")
	}

	color = color.downsampleTo(terminalColorCount)

	// We never create any ColorCount8 colors, but we store them as
	// ColorCount16. So this if() statement will cover both.
	if color.ColorCount() == ColorCount16 {
		if cType == colorTypeUnderline {
			// Only 256 and 24 bit colors supported for underline color
			return ""
		}

		value := color.colorValue()
		if value < 8 {
			return fmt.Sprint("\x1b[", typeMarker, value, "m")
		} else if value <= 15 {
			typeMarker := "9"
			if cType == colorTypeBackground {
				typeMarker = "10"
			}
			return fmt.Sprint("\x1b[", typeMarker, value-8, "m")
		}

		panic(fmt.Errorf("unhandled color16 value %d", value))
	}

	if color.ColorCount() == ColorCount256 {
		value := color.colorValue()
		if value <= 255 {
			return fmt.Sprint("\x1b[", typeMarker, "8;5;", value, "m")
		}
	}

	if color.ColorCount() == ColorCount24bit {
		value := color.colorValue()
		red := (value & 0xff0000) >> 16
		green := (value & 0xff00) >> 8
		blue := value & 0xff

		return fmt.Sprint("\x1b[", typeMarker, "8;2;", red, ";", green, ";", blue, "m")
	}

	panic(fmt.Errorf("unhandled color type=%d %s", color.ColorCount(), color.String()))
}

func (color Color) String() string {
	switch color.ColorCount() {
	case ColorCountDefault:
		return "Default color"

	case ColorCount16:
		return colorNames16[int(color.colorValue())]

	case ColorCount256:
		if color.colorValue() < 16 {
			return colorNames16[int(color.colorValue())]
		}
		return fmt.Sprintf("#%02x", color.colorValue())

	case ColorCount24bit:
		return fmt.Sprintf("#%06x", color.colorValue())
	}

	panic(fmt.Errorf("unhandled color type %d", color.ColorCount()))
}

func (color Color) to24Bit() Color {
	if color.ColorCount() == ColorCount24bit {
		return color
	}

	if color.ColorCount() == ColorCount8 || color.ColorCount() == ColorCount16 || color.ColorCount() == ColorCount256 {
		r0, g0, b0 := color256ToRGB(uint8(color.colorValue()))
		return NewColor24Bit(r0, g0, b0)
	}

	panic(fmt.Errorf("unhandled color type %d", color.ColorCount()))
}

func (color Color) downsampleTo(terminalColorCount ColorCount) Color {
	if color.ColorCount() == ColorCountDefault || terminalColorCount == ColorCountDefault {
		panic(fmt.Errorf("downsampling to or from default color not supported, %s -> %#v", color.String(), terminalColorCount))
	}

	if color.ColorCount() <= terminalColorCount {
		// Already low enough
		return color
	}

	target := color.to24Bit()

	// Find the closest match in the terminal color palette
	var scanFirst int
	var scanLast int
	switch terminalColorCount {
	case ColorCount8:
		scanFirst = 0
		scanLast = 7
	case ColorCount16:
		scanFirst = 0
		scanLast = 15
	case ColorCount256:
		// Colors 0-15 can be customized by the user, so we skip them and use
		// only the well defined ones
		scanFirst = 16
		scanLast = 255
	default:
		panic(fmt.Errorf("unhandled terminal color count %#v", terminalColorCount))
	}

	// Iterate over the scan range and find the best matching index
	bestMatch := 0
	bestDistance := math.MaxFloat64
	for i := scanFirst; i <= scanLast; i++ {
		r, g, b := color256ToRGB(uint8(i))
		candidate := NewColor24Bit(r, g, b)

		distance := target.Distance(candidate)
		if distance < bestDistance {
			bestDistance = distance
			bestMatch = i
		}
	}

	if bestMatch <= 15 {
		return NewColor16(bestMatch)
	}
	return NewColor256(uint8(bestMatch))
}

// Wrapper for Chroma's color distance function.
//
// That one says it uses this formula: https://www.compuphase.com/cmetric.htm
//
// The result from this function has been scaled to 0.0-1.0, where 1.0 is the
// distance between black and white.
func (color Color) Distance(other Color) float64 {
	if color == ColorDefault || other == ColorDefault {
		panic(fmt.Errorf("calculating distance to or from default color not supported, %s <-> %s", color.String(), other.String()))
	}

	color = color.to24Bit()
	other = other.to24Bit()

	baseColor := chroma.NewColour(
		uint8(color.colorValue()>>16&0xff),
		uint8(color.colorValue()>>8&0xff),
		uint8(color.colorValue()&0xff),
	)

	otherColor := chroma.NewColour(
		uint8(other.colorValue()>>16&0xff),
		uint8(other.colorValue()>>8&0xff),
		uint8(other.colorValue()&0xff),
	)

	// Magic constant comes from testing
	maxDistance := 764.8333151739665
	return baseColor.Distance(otherColor) / maxDistance
}

// With weight 0.0 you'll get only color. With weight 1.0 you'll get only other.
func (color Color) Mix(other Color, weight float64) Color {
	if color.ColorCount() == ColorCountDefault || other.ColorCount() == ColorCountDefault {
		panic(fmt.Errorf("mixing to or from default color not supported, %s <-> %s", color.String(), other.String()))
	}
	if weight < 0.0 || weight > 1.0 {
		panic(fmt.Errorf("weight must be 0.0-1.0, got %f", weight))
	}

	c1_24 := color.to24Bit()
	c2_24 := other.to24Bit()

	c1_value := c1_24.colorValue()
	c1_red := (c1_value & 0xff0000) >> 16
	c1_green := (c1_value & 0xff00) >> 8
	c1_blue := c1_value & 0xff

	c2_value := c2_24.colorValue()
	c2_red := (c2_value & 0xff0000) >> 16
	c2_green := (c2_value & 0xff00) >> 8
	c2_blue := c2_value & 0xff

	// Mix the channels separately
	mixed_red := uint8(math.Round(float64(c2_red)*weight + float64(c1_red)*(1-weight)))
	mixed_green := uint8(math.Round(float64(c2_green)*weight + float64(c1_green)*(1-weight)))
	mixed_blue := uint8(math.Round(float64(c2_blue)*weight + float64(c1_blue)*(1-weight)))

	return NewColor24Bit(mixed_red, mixed_green, mixed_blue)
}
