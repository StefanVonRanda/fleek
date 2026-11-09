package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Theme    ThemeConfig    `toml:"theme"`
	Behavior BehaviorConfig `toml:"behavior"`
	Layout   LayoutConfig   `toml:"layout"`
	Images   ImagesConfig   `toml:"images"`
	Places   []PlaceConfig  `toml:"places"`
}

type ThemeConfig struct {
	Background       string `toml:"background"`
	Sidebar          string `toml:"sidebar"`
	Topbar           string `toml:"topbar"`
	Status           string `toml:"status"`
	Panel            string `toml:"panel"`
	Accent           string `toml:"accent"`
	Selected         string `toml:"selected"`
	Text             string `toml:"text"`
	Muted            string `toml:"muted"`
	Faint            string `toml:"faint"`
	Dimmer           string `toml:"dimmer"`
	Hover            string `toml:"hover"`
	SidebarActive    string `toml:"sidebar_active"`
	SidebarText      string `toml:"sidebar_text"`
	SearchBackground string `toml:"search_background"`
}

type BehaviorConfig struct {
	ShowHidden       bool   `toml:"show_hidden"`
	Sort             string `toml:"sort"`
	DirectoriesFirst bool   `toml:"directories_first"`
}

type LayoutConfig struct {
	SidebarWidth int `toml:"sidebar_width"`
	CardWidth    int `toml:"card_width"`
	Gap          int `toml:"gap"`
}

type ImagesConfig struct {
	Protocol string `toml:"protocol"`
}

type PlaceConfig struct {
	Label string `toml:"label"`
	Path  string `toml:"path"`
	Icon  string `toml:"icon"`
	Color string `toml:"color"`
}

var appConfig = defaultConfig()

func defaultConfig() Config {
	return Config{
		Theme: ThemeConfig{
			Background:       "#1d1c27",
			Sidebar:          "#15141e",
			Topbar:           "#181721",
			Status:           "#13141c",
			Panel:            "#1b1a24",
			Accent:           "#7aa2f7",
			Selected:         "#232a3f",
			Text:             "#c0caf5",
			Muted:            "#9a97aa",
			Faint:            "#565f89",
			Dimmer:           "#3b394d",
			Hover:            "#3a3950",
			SidebarActive:    "#262536",
			SidebarText:      "#a9afd5",
			SearchBackground: "#23222f",
		},
		Behavior: BehaviorConfig{
			Sort:             "name",
			DirectoriesFirst: true,
		},
		Layout: LayoutConfig{
			SidebarWidth: 20,
			CardWidth:    25,
			Gap:          2,
		},
		Images: ImagesConfig{Protocol: "auto"},
	}
}

func defaultConfigPath() string {
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		return filepath.Join(value, "fleek", "config.toml")
	}
	return filepath.Join(userHome(), ".config", "fleek", "config.toml")
}

func loadConfig(explicit string) (Config, string, error) {
	path := explicit
	if path == "" {
		path = os.Getenv("FLEEK_CONFIG")
	}
	if path == "" {
		path = defaultConfigPath()
	}
	config := defaultConfig()
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, path, nil
		}
		return config, path, err
	}
	if info.IsDir() {
		return config, path, fmt.Errorf("config path is a directory")
	}
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return defaultConfig(), path, err
	}
	return config, path, nil
}

func applyConfig(config Config) {
	applyTheme(config.Theme)
	appConfig = config

	switch strings.ToLower(config.Behavior.Sort) {
	case "size", "modified", "name":
		sortMode = strings.ToLower(config.Behavior.Sort)
	default:
		sortMode = "name"
	}
	dirsFirst = config.Behavior.DirectoriesFirst

	if config.Layout.SidebarWidth >= 14 && config.Layout.SidebarWidth <= 32 {
		sidebarW = config.Layout.SidebarWidth
	}
	if config.Layout.CardWidth >= 25 && config.Layout.CardWidth <= 40 {
		minCardW = config.Layout.CardWidth
	}
	if config.Layout.Gap >= 1 && config.Layout.Gap <= 4 {
		gapX = config.Layout.Gap
		gapY = config.Layout.Gap
	}
}

func applyTheme(theme ThemeConfig) {
	setColor := func(target *RGB, value string) {
		if color, ok := parseHex(value); ok {
			*target = color
		}
	}
	setColor(&colBG, theme.Background)
	setColor(&colSidebar, theme.Sidebar)
	setColor(&colTop, theme.Topbar)
	setColor(&colStatus, theme.Status)
	setColor(&colPanel, theme.Panel)
	setColor(&colAccent, theme.Accent)
	setColor(&colSelected, theme.Selected)
	setColor(&colText, theme.Text)
	setColor(&colMuted, theme.Muted)
	setColor(&colFaint, theme.Faint)
	setColor(&colDimmer, theme.Dimmer)
	setColor(&colHover, theme.Hover)
	setColor(&colSidebarActive, theme.SidebarActive)
	setColor(&colSidebarText, theme.SidebarText)
	setColor(&colSearchBG, theme.SearchBackground)
}

func expandPath(value string) string {
	value = os.ExpandEnv(strings.TrimSpace(value))
	if value == "~" {
		value = userHome()
	} else if strings.HasPrefix(value, "~/") {
		value = filepath.Join(userHome(), value[2:])
	}
	if value == "" {
		return ""
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return value
	}
	return absolute
}
