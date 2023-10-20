package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nxadm/tail"
	bsrpc "github.com/vinegarhq/vinegar/bloxstraprpc"
	"github.com/vinegarhq/vinegar/internal/config"
	"github.com/vinegarhq/vinegar/internal/config/state"
	"github.com/vinegarhq/vinegar/internal/dirs"
	"github.com/vinegarhq/vinegar/internal/splash"
	"github.com/vinegarhq/vinegar/roblox"
	"github.com/vinegarhq/vinegar/roblox/bootstrapper"
	"github.com/vinegarhq/vinegar/util"
	"github.com/vinegarhq/vinegar/wine"
	"github.com/vinegarhq/vinegar/wine/dxvk"
)

type Binary struct {
	Splash *splash.Splash

	GlobalConfig *config.Config
	Config       *config.Binary

	Alias   string
	Name    string
	Dir     string
	Prefix  *wine.Prefix
	Type    roblox.BinaryType
	Version roblox.Version
	Started time.Time
}

func NewBinary(bt roblox.BinaryType, cfg *config.Config, pfx *wine.Prefix) Binary {
	var bcfg config.Binary

	switch bt {
	case roblox.Player:
		bcfg = cfg.Player
	case roblox.Studio:
		bcfg = cfg.Studio
	}

	return Binary{
		Splash: splash.New(&cfg.Splash),

		GlobalConfig: cfg,
		Config:       &bcfg,

		Alias:  bt.String(),
		Name:   bt.BinaryName(),
		Type:   bt,
		Prefix: pfx,
	}
}

func (b *Binary) Run(args ...string) error {
	b.Splash.Log("")

	cmd, err := b.Command(args...)
	if err != nil {
		return err
	}

	log.Printf("Launching %s", b.Name)
	b.Splash.Message("Launching " + b.Alias)

	kill := true

	// If roblox is already running, don't kill wineprefix, even if
	// auto kill prefix is enabled
	if util.CommFound("Roblox") {
		log.Println("Roblox is already running, not killing wineprefix after exit")
		b.Splash.Log("Roblox is already running")
		kill = false
	}

	// Launches into foreground
	b.Started = time.Now()
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Process.Kill()

	if b.Config.DiscordRPC {
		err := bsrpc.Login()
		if err != nil {
			log.Printf("Failed to authenticate Discord RPC: %s, disabling RPC", err)
			b.Config.DiscordRPC = false
		}
		// this will fucking panic if it fails smh
		defer bsrpc.Logout()
	}

	p, err := b.FindLog()
	if err != nil {
		log.Printf("%s, has roblox successfully started?", err)
	} else {
		go func() {
			b.TailLog(p)
		}()
	}

	// after the FindLog function fails or found a log, we check if the
	// command process pid exists, we know by then if roblox had started
	// correctly, and to close the splash screen with the appropiate error.
	if cmd.Process.Pid != 0 {
		b.Splash.Close()
	}

	defer func() {
		if kill && b.Config.AutoKillPrefix {
			b.Prefix.Kill()
		}
	}()

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func (b *Binary) FindLog() (string, error) {
	appData, err := b.Prefix.AppDataDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(appData, "Local", "Roblox", "logs")
	// May not exist if roblox has its first run
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	log.Println("Polling for Roblox log file, 10 retries")
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)

		name, err := util.FindTimeFile(dir, &b.Started)
		if err == nil {
			log.Printf("Found Roblox log file: %s", name)
			return name, nil
		}
	}

	return "", fmt.Errorf("could not roblox log file after time %s", b.Started)
}

func (b *Binary) TailLog(name string) {
	var a bsrpc.Activity

	t, err := tail.TailFile(name, tail.Config{Follow: true})
	if err != nil {
		log.Printf("Failed to tail Roblox log file: %s", err)
		return
	}

	for line := range t.Lines {
		fmt.Fprintln(b.Prefix.Output, line.Text)

		if b.Config.DiscordRPC {
			if err := a.HandleLog(line.Text); err != nil {
				log.Printf("Failed to handle Discord RPC: %s", err)
			}
		}
	}
}

func (b *Binary) FetchVersion() (roblox.Version, error) {
	b.Splash.Message("Fetching " + b.Alias)

	if b.Config.ForcedVersion != "" {
		log.Printf("WARNING: using forced version: %s", b.Config.ForcedVersion)

		return roblox.NewVersion(b.Type, b.Config.Channel, b.Config.ForcedVersion)
	}

	return roblox.LatestVersion(b.Type, b.Config.Channel)
}

func (b *Binary) Setup() error {
	ver, err := b.FetchVersion()
	if err != nil {
		return err
	}

	b.Splash.Desc(ver.String())
	b.Version = ver
	b.Dir = filepath.Join(dirs.Versions, ver.GUID)

	stateVer, err := state.Version(b.Type)
	if err != nil {
		log.Printf("Failed to retrieve stored %s version: %s", b.Name, err)
	}

	stateVer = ""

	if stateVer != ver.GUID {
		log.Printf("Installing %s (%s -> %s)", b.Name, stateVer, ver.GUID)

		if err := b.Install(); err != nil {
			return err
		}
	} else {
		b.Splash.Log("Up to date")
		log.Printf("%s is up to date (%s)", b.Name, ver.GUID)
	}

	b.Config.Env.Setenv()

	b.Splash.Log("Setting Renderer")
	if err := b.Config.FFlags.SetRenderer(b.Config.Renderer); err != nil {
		return err
	}

	b.Splash.Log("Applying FFlags")
	if err := b.Config.FFlags.Apply(b.Dir); err != nil {
		return err
	}

	b.Splash.Log("Applying Overlay modifications")
	if err := dirs.OverlayDir(b.Dir); err != nil {
		return err
	}

	if err := b.SetupDxvk(); err != nil {
		return err
	}

	b.Splash.Progress(1.0)
	return nil
}

func (b *Binary) Install() error {
	b.Splash.Message("Installing " + b.Alias)

	if err := dirs.Mkdirs(dirs.Downloads); err != nil {
		return err
	}

	b.Splash.Log("Fetching package manifest")
	manifest, err := bootstrapper.FetchManifest(&b.Version)
	if err != nil {
		return err
	}

	if err := dirs.Mkdirs(dirs.Downloads); err != nil {
		return err
	}

	b.Splash.Message("Downloading " + b.Alias)
	if err := b.DownloadPackages(&manifest); err != nil {
		return err
	}

	b.Splash.Message("Extracting " + b.Alias)
	if err := b.ExtractPackages(&manifest); err != nil {
		return err
	}

	if b.Type == roblox.Studio {
		brokenFont := filepath.Join(b.Dir, "StudioFonts", "SourceSansPro-Black.ttf")

		log.Printf("Removing broken font %s", brokenFont)
		if err := os.RemoveAll(brokenFont); err != nil {
			log.Printf("Failed to remove font: %s", err)
		}
	}

	b.Splash.Log("Writing AppSettings")
	if err := bootstrapper.WriteAppSettings(b.Dir); err != nil {
		return err
	}

	if err := state.SaveManifest(&manifest); err != nil {
		return err
	}

	b.Splash.Log("Cleaning up")
	if err := state.CleanPackages(); err != nil {
		return err
	}

	return state.CleanVersions()
}

func (b *Binary) PerformPackages(m *bootstrapper.Manifest, fn func(bootstrapper.Package) error) error {
	donePkgs := 0
	pkgsLen := len(m.Packages)

	return m.Packages.Perform(func(pkg bootstrapper.Package) error {
		b.Splash.Log(pkg.Name)

		err := fn(pkg)
		if err != nil {
			return err
		}

		donePkgs++
		b.Splash.Progress(float32(donePkgs) / float32(pkgsLen))
		b.Splash.Log(pkg.Name)

		return nil
	})
}

func (b *Binary) DownloadPackages(m *bootstrapper.Manifest) error {
	log.Printf("Downloading %d Packages for %s", len(m.Packages), m.Version.GUID)

	return b.PerformPackages(m, func(pkg bootstrapper.Package) error {
		return pkg.Fetch(filepath.Join(dirs.Downloads, pkg.Checksum), m.DeployURL)
	})
}

func (b *Binary) ExtractPackages(m *bootstrapper.Manifest) error {
	log.Printf("Extracting %d Packages for %s", len(m.Packages), m.Version.GUID)
	pkgDirs := bootstrapper.BinaryDirectories(b.Type)

	return b.PerformPackages(m, func(pkg bootstrapper.Package) error {
		dest, ok := pkgDirs[pkg.Name]

		if !ok {
			return fmt.Errorf("unhandled package: %s", pkg.Name)
		}

		return pkg.Extract(filepath.Join(dirs.Downloads, pkg.Checksum), filepath.Join(b.Dir, dest))
	})
}

func (b *Binary) SetupDxvk() error {
	ver, err := state.DxvkVersion()
	if err != nil {
		return err
	}
	installed := ver != ""

	if installed && !b.GlobalConfig.Player.Dxvk && !b.GlobalConfig.Studio.Dxvk {
		b.Splash.Message("Uninstalling DXVK")
		if err := dxvk.Remove(b.Prefix); err != nil {
			return err
		}

		return state.SaveDxvk("")
	}

	if !b.Config.Dxvk {
		return nil
	}

	b.Splash.Progress(0.0)
	dxvk.Setenv()

	if b.GlobalConfig.DxvkVersion == ver {
		return nil
	}

	if err := dirs.Mkdirs(dirs.Cache); err != nil {
		return err
	}
	path := filepath.Join(dirs.Cache, "dxvk-"+b.GlobalConfig.DxvkVersion+".tar.gz")

	b.Splash.Progress(0.3)
	b.Splash.Message("Downloading DXVK")
	if err := dxvk.Fetch(path, b.GlobalConfig.DxvkVersion); err != nil {
		return err
	}

	b.Splash.Progress(0.7)
	b.Splash.Message("Extracting DXVK")
	if err := dxvk.Extract(path, b.Prefix); err != nil {
		return err
	}
	b.Splash.Progress(1.0)

	return state.SaveDxvk(b.GlobalConfig.DxvkVersion)
}

func (b *Binary) Command(args ...string) (*wine.Cmd, error) {
	if strings.HasPrefix(strings.Join(args, " "), "roblox-studio:1") {
		args = []string{"-protocolString", args[0]}
	}

	if b.GlobalConfig.MultipleInstances {
		b.Splash.Log("Launching robloxmutexer")

		mutexer := b.Prefix.Command("wine", filepath.Join(BinPrefix, "robloxmutexer.exe"))
		err := mutexer.Start()
		if err != nil {
			return &wine.Cmd{}, err
		}
	}

	exe := filepath.Join(b.Dir, b.Type.Executable())
	cmd := b.Prefix.Wine(exe, args...)

	launcher := strings.Fields(b.Config.Launcher)
	if len(launcher) >= 1 {
		cmd.Args = append(launcher, cmd.Args...)

		launcherPath, err := exec.LookPath(launcher[0])
		if err != nil {
			return &wine.Cmd{}, err
		}

		cmd.Path = launcherPath
	}

	return cmd, nil
}
