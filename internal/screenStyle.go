package internal

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/walles/moor/v2/twin"
)

const defaultDarkTheme = "native"

// I decided on a light theme by doing this:
//
//	rg -i 'Background.*bg:#ffffff' | rg -v '[^:]#[^0]' | sort | cut -d: -f1 | xargs rg --files-without-match '"LiteralString".*bg:' |  xargs wc -l | sort -r | grep -v tango
//
// Then I picked github because it has a bright background, a dark foreground
// and it looks OK on a white terminal with an unmodified color palette.
//
// This used to be tango, which I mostly like, but it comes with underlined
// whitespace which looks weird when we change the background color of lines
// with search hits:
//
// https://github.com/alecthomas/chroma/blob/daa879b239442af21c3e62517a9da8f11d1c15b2/styles/tango.xml#L71
const defaultLightTheme = "github"

// Checks the terminal background color and returns either a dark or light theme
func GetStyleForScreen(screen twin.Screen) chroma.Style {
	bgColor := screen.TerminalBackground()
	if bgColor == nil {
		// Fall back to dark theme if we can't detect the background color
		return *styles.Get(defaultDarkTheme)
	}

	distanceToBlack := bgColor.Distance(twin.NewColor24Bit(0, 0, 0))
	distanceToWhite := bgColor.Distance(twin.NewColor24Bit(255, 255, 255))
	if distanceToBlack < distanceToWhite {
		return *styles.Get(defaultDarkTheme)
	} else {
		return *styles.Get(defaultLightTheme)
	}
}
