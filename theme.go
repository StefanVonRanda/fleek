package main

import "fmt"

type RGB struct {
	R, G, B uint8
}

func hexRGB(value string) RGB {
	var r, g, b uint8
	if _, err := fmt.Sscanf(value, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return RGB{255, 255, 255}
	}
	return RGB{r, g, b}
}

var (
	colBG            = hexRGB("#1d1c27")
	colSidebar       = hexRGB("#15141e")
	colTop           = hexRGB("#181721")
	colStatus        = hexRGB("#13141c")
	colPanel         = hexRGB("#1b1a24")
	colText          = hexRGB("#c0caf5")
	colMuted         = hexRGB("#9a97aa")
	colFaint         = hexRGB("#565f89")
	colDimmer        = hexRGB("#3b394d")
	colAccent        = hexRGB("#7aa2f7")
	colSelected      = hexRGB("#232a3f")
	colHover         = hexRGB("#3a3950")
	colTint          = hexRGB("#201f2c")
	colRed           = hexRGB("#f7768e")
	colGreen         = hexRGB("#9ece6a")
	colAmber         = hexRGB("#e0af68")
	colCyan          = hexRGB("#7dcfff")
	colPurple        = hexRGB("#bb9af7")
	colOrange        = hexRGB("#ff9e64")
	colSidebarActive = hexRGB("#262536")
	colSidebarText   = hexRGB("#a9afd5")
	colSearchBG      = hexRGB("#23222f")
)

func scale(c RGB, factor float64) RGB {
	return RGB{
		R: uint8(float64(c.R) * factor),
		G: uint8(float64(c.G) * factor),
		B: uint8(float64(c.B) * factor),
	}
}

func blend(a, b RGB, amount float64) RGB {
	return RGB{
		R: uint8(float64(a.R)*(1-amount) + float64(b.R)*amount),
		G: uint8(float64(a.G)*(1-amount) + float64(b.G)*amount),
		B: uint8(float64(a.B)*(1-amount) + float64(b.B)*amount),
	}
}
