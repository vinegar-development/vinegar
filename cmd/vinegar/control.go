package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/gio"
	"github.com/jwijenbergh/puregotk/v4/glib"
	_ "github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/gtk"
	"github.com/vinegarhq/vinegar/internal/dirs"
)

type control struct {
	*ui

	builder *gtk.Builder
	win     adw.ApplicationWindow
}

func (s *ui) NewControl() control {
	ctl := control{
		ui:      s,
		builder: gtk.NewBuilderFromResource("/org/vinegarhq/Vinegar/ui/control.ui"),
	}

	ctl.builder.GetObject("window").Cast(&ctl.win)
	ctl.win.SetApplication(&s.app.Application)

	abt := gio.NewSimpleAction("about", nil)
	abtcb := func(_ gio.SimpleAction, p uintptr) {
		ctl.PresentAboutWindow()
	}
	abt.ConnectActivate(&abtcb)
	ctl.app.AddAction(abt)
	abt.Unref()

	ctl.PutConfig()
	ctl.UpdateButtons()

	// For the time being, use in-house editing.
	// ctl.SetupConfigurationActions()
	ctl.SetupControlActions()

	ctl.win.Present()
	ctl.win.Unref()

	return ctl
}

func (ctl *control) PutConfig() {
	var view gtk.TextView
	ctl.builder.GetObject("config-view").Cast(&view)

	b, err := os.ReadFile(dirs.ConfigPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		b = []byte(err.Error())
	}

	view.GetBuffer().SetText(string(b), -1)
}

func (ctl *control) SetupControlActions() {
	actions := map[string]struct {
		msg string
		act interface{}
	}{
		"run-studio": {"Executing Studio", (*bootstrapper).Run},
		"run-winetricks": {"Executing Winetricks", func() error {
			return ctl.pfx.Tricks().Run()
		}},

		"install-studio":   {"Installing Studio", (*bootstrapper).Setup},
		"uninstall-studio": {"Deleting all deployments", ctl.DeleteDeployments},

		"init-prefix": {"Initializing Wineprefix", func() error {
			return WineSimpleRun(ctl.pfx.Init())
		}},
		"kill-prefix":   {"Killing Wineprefix", ctl.pfx.Kill},
		"delete-prefix": {"Deleting Wineprefix", ctl.DeletePrefixes},

		"save-config": {"Saving configuration to file", ctl.SaveConfig},
		"clear-cache": {"Cleaning up cache folder", ctl.ClearCacheDir},
	}

	var stack adw.ViewStack
	ctl.builder.GetObject("stack").Cast(&stack)
	var label gtk.Label
	ctl.builder.GetObject("loading-label").Cast(&label)

	// Reserved only for execution of studio
	var btn gtk.Button
	ctl.builder.GetObject("loading-stop").Cast(&btn)
	stop := gio.NewSimpleAction("show-stop", nil)
	stopcb := func(_ gio.SimpleAction, p uintptr) {
		btn.SetVisible(true)
	}
	stop.ConnectActivate(&stopcb)
	ctl.app.AddAction(stop)
	stop.Unref()

	for name, action := range actions {
		act := gio.NewSimpleAction(name, nil)
		actcb := func(_ gio.SimpleAction, p uintptr) {
			stack.SetVisibleChildName("loading")
			label.SetLabel(action.msg + "...")
			btn.SetVisible(false)

			var run func() error
			switch f := action.act.(type) {
			case func() error:
				run = func() error {
					slog.Info(action.msg + "...")
					return f()
				}
			case func(*bootstrapper) error:
				run = func() error {
					b := ctl.NewBootstrapper()
					b.win.SetTransientFor(&ctl.win.Window)
					defer Background(b.win.Destroy)
					return f(b)
				}
			default:
				panic("unreachable")
			}

			var tf glib.ThreadFunc
			tf = func(uintptr) uintptr {
				defer Background(func() {
					ctl.UpdateButtons()
					stack.SetVisibleChildName("control")
				})
				if err := run(); err != nil {
					Background(func() { ctl.presentSimpleError(err) })
				}
				return null
			}
			glib.NewThread("action", &tf, null)
		}

		act.ConnectActivate(&actcb)
		ctl.app.AddAction(act)
		act.Unref()
	}
}

/*
func (ctl *control) SetupConfigurationActions() {
	props := []struct {
		name   string
		widget string
		modify any
	}{
		{"gamemode", "switch", ctl.cfg.Studio.GameMode},
		{"launcher", "entry", ctl.cfg.Studio.Launcher},
	}

	save := func() {
		if err := ctl.LoadConfig(); err != nil {
			ctl.presentSimpleError(err)
		}
	}

	for _, p := range props {
		obj := ctl.builder.GetObject(p.name)

		switch p.widget {
		case "switch":
			// AdwSwitchRow does not implement Gtk.Switch, and neither
			// does puregotk have a reference for AdwSwitchRow.
			actcb := func() {
				var new bool
				obj.Get("active", &new)
				p.modify = new
				slog.Info("Configuration Switch", "name", p.widget, "value", p.modify)
				save()
			}
			var w gtk.Widget
			obj.Cast(&w)
			obj.ConnectSignal("notify::active", &actcb)
			slog.Info("Setup Widget", "name", p.name)
		case "entry":
			var entry adw.EntryRow
			obj.Cast(&entry)

			cb := func(_ adw.EntryRow) {
				p.modify = entry.GetText()
				slog.Info("Configuration Entry", "name", p.widget, "value", p.modify)
				save()
			}
			entry.ConnectApply(&cb)
		default:
			panic("unreachable")
		}
	}

	// this is a special button!
	var rootRow adw.ActionRow
	ctl.builder.GetObject("wineroot-row").Cast(&rootRow)
	rootRow.SetSubtitle(ctl.cfg.Studio.WineRoot)

	var rootSelect gtk.Button
	ctl.builder.GetObject("wineroot-select").Cast(&rootSelect)
	actcb := func(_ gtk.Button) {
		// gtk.FileChooser is deprecated for gtk.FileDialog, but puregotk does not have it.
		fc := gtk.NewFileChooserDialog("Select Wine Installation", &ctl.win.Window,
			gtk.FileChooserActionSelectFolderValue,
			"Cancel", gtk.ResponseCancelValue,
			"Select", gtk.ResponseAcceptValue,
			unsafe.Pointer(nil),
		)
		rcb := func(_ gtk.Dialog, _ int) {
			cp := ctl.cfg.Studio
			cp.WineRoot = fc.GetFile().GetPath()
			// if err := cp.Setup(); err != nil {
			rootRow.SetSubtitle(ctl.cfg.Studio.WineRoot)
			fc.Destroy()
		}
		fc.ConnectResponse(&rcb)
		fc.Present()
	}
	rootSelect.ConnectClicked(&actcb)
}
*/

func (ctl *control) SaveConfig() error {
	var view gtk.TextView
	var start, end gtk.TextIter
	ctl.builder.GetObject("config-view").Cast(&view)

	buf := view.GetBuffer()
	buf.GetBounds(&start, &end)
	text := buf.GetText(&start, &end, false)

	if err := dirs.Mkdirs(dirs.Config); err != nil {
		return err
	}

	f, err := os.OpenFile(dirs.ConfigPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write([]byte(text)); err != nil {
		return err
	}

	return ctl.LoadConfig()
}

func (ui *ui) DeleteDeployments() error {
	if err := os.RemoveAll(dirs.Versions); err != nil {
		return err
	}

	ui.state.Studio.Version = ""
	ui.state.Studio.Packages = nil

	return ui.state.Save()
}

func (ctl *control) DeletePrefixes() error {
	slog.Info("Deleting Wineprefixes!")

	if err := ctl.pfx.Kill(); err != nil {
		return fmt.Errorf("kill prefix: %w", err)
	}

	if err := os.RemoveAll(dirs.Prefixes); err != nil {
		return err
	}

	ctl.state.Studio.DxvkVersion = ""

	if err := ctl.state.Save(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	return nil
}

func (ctl *control) ClearCacheDir() error {
	return filepath.WalkDir(dirs.Cache, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dirs.Cache || path == dirs.Logs || path == ctl.logFile.Name() {
			return nil
		}

		slog.Info("Removing cache file", "path", path)
		return os.RemoveAll(path)
	})
}

func (ctl *control) UpdateButtons() {
	pfx := ctl.pfx.Exists()
	vers := dirs.Empty(dirs.Versions)

	// While kill-prefix is more of a wineprefix-specific action,
	// it is instead listed as an option belonging to the Studio
	// area, to indicate that it is used to kill studio.
	var inst, uninst, run gtk.Widget
	ctl.builder.GetObject("studio-install").Cast(&inst)
	ctl.builder.GetObject("studio-uninstall").Cast(&uninst)
	ctl.builder.GetObject("studio-run").Cast(&run)
	inst.SetVisible(vers)
	uninst.SetVisible(!vers)
	run.SetVisible(!vers)

	var init, kill, del, tricks gtk.Widget
	ctl.builder.GetObject("prefix-init").Cast(&init)
	ctl.builder.GetObject("prefix-kill").Cast(&kill)
	ctl.builder.GetObject("prefix-delete").Cast(&del)
	ctl.builder.GetObject("prefix-winetricks").Cast(&tricks)
	init.SetVisible(!pfx)
	del.SetVisible(pfx)
	kill.SetVisible(pfx)
	tricks.SetVisible(pfx)
}

func (ctl *control) PresentAboutWindow() {
	w := adw.NewAboutWindow()
	defer w.Unref()

	w.SetApplicationName("Vinegar")
	w.SetApplicationIcon("org.vinegarhq.Vinegar")
	w.SetIssueUrl("https://github.com/vinegarhq/vinegar/issues")
	w.SetSupportUrl("https://discord.gg/dzdzZ6Pps2")
	w.SetWebsite("https://vinegarhq.org")
	w.SetLicenseType(gtk.LicenseAgpl30OnlyValue)
	w.SetVersion(Version)
	w.SetDebugInfo(ctl.DebugInfo())
	w.SetTransientFor(&ctl.win.Window)
	w.Present()
}
