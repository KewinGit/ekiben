package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/KewinGit/ekiben/internal/config"
	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/KewinGit/ekiben/internal/ui"
	"github.com/KewinGit/ekiben/internal/version"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfgPath := flag.String("config", config.Path(), "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("ekiben", version.String())
		return
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
	}

	cli, err := docker.NewSDK()
	if err != nil {
		fmt.Fprintln(os.Stderr, "docker:", err)
		os.Exit(1)
	}
	defer cli.Close()

	m := ui.New(cli, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ekiben:", err)
		os.Exit(1)
	}
}
