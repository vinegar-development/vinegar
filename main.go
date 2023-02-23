// Copyright vinegar-development 2023

package main

import (
	"fmt"
	"os"
)

const (
	PLAYERURL = "https://www.roblox.com/download/client"
	STUDIOURL = "https://www.roblox.com/download/studio"
)

func usage() {
	fmt.Println("usage: vinegar [delete|edit|exec|kill|reset]")
	fmt.Println("       vinegar [player|studio] [args...]")
	if !InFlatpak {
		fmt.Println("       vinegar [dxvk] install|uninstall")
	}
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	argsCount := len(args)

	if argsCount < 1 {
		usage()
	}

	// Only these Dirs are queued for creation since
	// the other directories are root directories for those.
	CheckDirs(0755, Dirs.Log, Dirs.Pfx)

	switch args[0] {
	case "delete":
		EdgeDirSet(0755, false)
		DeleteDirs(Dirs.Data, Dirs.Cache)
	case "dxvk":
		if argsCount < 2 {
			usage()
		}

		switch args[1] {
		case "install":
			DxvkInstall()
		case "uninstall":
			DxvkUninstall()
		default:
			usage()
		}
	case "edit":
		EditConfig()
	case "exec":
		Exec("wine", false, args[1:]...)
	case "kill":
		PfxKill()
	case "player":
		RobloxLaunch("RobloxPlayerLauncher.exe", "Client", PLAYERURL, args[1:]...)
		// Wait for the RobloxPlayerLauncher to exit, and that is because Roblox
		// may update, so we wait for it to exit, and proceed with waiting for the
		// preceeding fork of the launcher to the player, and then kill the prefix.
		CommLoop("RobloxPlayerBet")
		PfxKill()
	case "studio":
		RobloxLaunch("RobloxStudioLauncherBeta.exe", "Studio", STUDIOURL, args[1:]...)
		// Same thing as player, behavior is subject to change.
		CommLoop("RobloxStudioBet")
		PfxKill()
	case "reset":
		EdgeDirSet(0755, false)
		DeleteDirs(Dirs.Pfx, Dirs.Log)
		// Automatic creation of the directories after it has been deleted
		CheckDirs(0755, Dirs.Pfx, Dirs.Log)
	default:
		usage()
	}
}
