package main

import (
	"strconv"
	"strings"
)

type RGB struct {
	R, G, B uint8
}

func parseHex(value string) (RGB, bool) {
	value = strings.TrimSpace(value)
	if len(value) == 4 && value[0] == '#' {
		value = strings.Join([]string{"#", string(value[1]), string(value[1]), string(value[2]), string(value[2]), string(value[3]), string(value[3])}, "")
	}
	if len(value) != 7 || value[0] != '#' {
		return RGB{255, 255, 255}, false
	}
	number, err := strconv.ParseUint(value[1:], 16, 24)
	if err != nil {
		return RGB{255, 255, 255}, false
	}
	return RGB{uint8(number >> 16), uint8(number >> 8), uint8(number)}, true
}

func hexRGB(value string) RGB {
	color, ok := parseHex(value)
	if !ok {
		return RGB{255, 255, 255}
	}
	return color
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
