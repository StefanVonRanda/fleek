package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

const version = "0.3.0"

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: fleek [folder]\n")
	}
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println("fleek", version)
		return
	}
	path := "."
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	program := tea.NewProgram(newModel(path), tea.WithColorProfile(colorprofile.TrueColor))
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
