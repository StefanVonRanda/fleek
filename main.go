package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

const version = "0.8.0"

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: fleek [folder]\n")
	}
	showVersion := flag.Bool("version", false, "print version")
	configFlag := flag.String("config", "", "path to a TOML config file")
	flag.Parse()
	if *showVersion {
		fmt.Println("fleek", version)
		return
	}
	config, configPath, err := loadConfig(*configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fleek: %s: %v\n", configPath, err)
	}
	applyConfig(config)
	path := "."
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	program := tea.NewProgram(newModel(path), tea.WithColorProfile(colorprofile.TrueColor))
	setShiftCapture(true)
	_, err = program.Run()
	setShiftCapture(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
