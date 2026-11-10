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
	colBG            = hexRGB("#1e1e2e")
	colSidebar       = hexRGB("#181825")
	colTop           = hexRGB("#181825")
	colStatus        = hexRGB("#1e1e2e")
	colPanel         = hexRGB("#1e1e2e")
	colText          = hexRGB("#cdd6f4")
	colMuted         = hexRGB("#a6adc8")
	colFaint         = hexRGB("#6c7086")
	colDimmer        = hexRGB("#585b70")
	colAccent        = hexRGB("#cba6f7")
	colSelected      = hexRGB("#313244")
	colHover         = hexRGB("#45475a")
	colTint          = hexRGB("#242438")
	colRed           = hexRGB("#f38ba8")
	colGreen         = hexRGB("#a6e3a1")
	colAmber         = hexRGB("#f9e2af")
	colCyan          = hexRGB("#89dceb")
	colPurple        = hexRGB("#b4befe")
	colOrange        = hexRGB("#fab387")
	colSidebarActive = hexRGB("#313244")
	colSidebarText   = hexRGB("#a6adc8")
	colSearchBG      = hexRGB("#313244")
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
