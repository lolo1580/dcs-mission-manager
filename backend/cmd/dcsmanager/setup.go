package main

import (
	"dcsmanager/internal/config"
	"dcsmanager/internal/dcsdir"
	"dcsmanager/internal/install"
	"dcsmanager/internal/userinstall"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf16"
)

func checkSetup(withLua bool) error {
	running, err := install.ManagerRunning()
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("ferme DCS Manager et sa version portable avant de poursuivre")
	}
	if withLua {
		running, err = install.DCSRunning()
		if err != nil {
			return err
		}
		if running {
			return fmt.Errorf("ferme DCS avant d’installer ses scripts Lua")
		}
	}
	return nil
}
func runSetupCheck(args []string) int {
	flags := flag.NewFlagSet("setup-check", flag.ContinueOnError)
	lua := flags.Bool("install-lua", false, "check DCS too")
	if flags.Parse(args) != nil {
		return 1
	}
	if err := checkSetup(*lua); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
func runSetupUser(args []string) int {
	flags := flag.NewFlagSet("setup-user", flag.ContinueOnError)
	sg := flags.String("saved-games", "", "DCS Saved Games folder")
	portable := flags.String("portable-dir", "", "portable application folder to copy")
	dcsInstall := flags.String("dcs-install", "", "DCS game installation root")
	root := flags.String("data-root", "", "persistent user data root")
	lua := flags.Bool("install-lua", false, "install bundled Lua scripts, preserving settings")
	if flags.Parse(args) != nil {
		return 1
	}
	if *root == "" {
		var err error
		*root, err = config.UserDataRoot()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	if err := os.MkdirAll(*root, 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var output io.Writer = os.Stderr
	if file, err := os.OpenFile(filepath.Join(*root, "setup.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		defer file.Close()
		output = io.MultiWriter(os.Stderr, file)
	}
	fail := func(err error) int { fmt.Fprintln(output, err); return 1 }
	if err := checkSetup(*lua); err != nil {
		return fail(err)
	}
	if *lua && *sg == "" {
		return fail(fmt.Errorf("dossier DCS requis pour installer Lua"))
	}
	note, err := userinstall.Configure(*root, *sg, *portable, *dcsInstall)
	if err != nil {
		return fail(err)
	}
	fmt.Fprintln(output, note)
	if *lua {
		results, err := install.New("", *sg).InstallPreservingConfig()
		for _, result := range results {
			fmt.Fprintf(output, "%s %s backup=%s\n", result.Action, result.DestRel, result.Backup)
		}
		if err != nil {
			return fail(err)
		}
	}
	return 0
}

func runSetupDetect(args []string) int {
	flags := flag.NewFlagSet("setup-detect", flag.ContinueOnError)
	output := flags.String("output", "", "INI file for the setup wizard")
	if flags.Parse(args) != nil || *output == "" {
		return 1
	}
	cfg := config.Load()
	if root, err := config.UserDataRoot(); err == nil {
		if saved, err := config.ReadUserSettings(root); err == nil {
			config.ApplyUserPaths(&cfg, saved)
		}
	}
	game := cfg.DCSInstall
	if game == "" {
		game = dcsdir.Find(cfg.SavedGames)
	}
	text := "[DCS]\r\nSavedGames=" + cfg.SavedGames + "\r\nGame=" + game + "\r\n"
	// Windows INI APIs read Unicode files as UTF-16 LE with a BOM. UTF-8
	// would corrupt accented usernames and selected folders in the wizard.
	units := utf16.Encode([]rune(text))
	data := make([]byte, 2+len(units)*2)
	data[0], data[1] = 0xff, 0xfe
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[2+i*2:], unit)
	}
	if err := os.WriteFile(*output, data, 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
