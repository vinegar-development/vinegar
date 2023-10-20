//go:build !nogui && !nosplash

package splash

import (
	"bytes"
	"errors"
	"image"
	_ "image/png"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/vinegarhq/vinegar/internal/config"
	"github.com/vinegarhq/vinegar/util"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

var ErrClosed = errors.New("window closed")

type Splash struct {
	*app.Window

	Theme  *material.Theme
	Config *config.Splash

	logo     image.Image
	message  string
	log      string
	desc     string
	showLog  string
	progress float32
	closed   bool
}

func (ui *Splash) Message(msg string) {
	ui.message = msg
	ui.Invalidate()
}

func (ui *Splash) Log(msg string) {
	ui.log = msg
	ui.Invalidate()
}

func (ui *Splash) Desc(desc string) {
	ui.desc = desc
	ui.Invalidate()
}

func (ui *Splash) ShowLog(name string) {
	ui.showLog = name
	ui.Invalidate()
}

func (ui *Splash) Progress(progress float32) {
	ui.progress = progress
	ui.Invalidate()
}

func (ui *Splash) Close() {
	ui.closed = true
	ui.Perform(system.ActionClose)
}

func New(cfg *config.Splash) *Splash {
	width := unit.Dp(448)
	height := unit.Dp(240)

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Palette = material.Palette{
		Bg:         rgb(cfg.Bg),
		Fg:         rgb(cfg.Fg),
		ContrastBg: rgb(cfg.Accent),
		ContrastFg: rgb(cfg.Gray2),
	}

	logo, _, _ := image.Decode(bytes.NewReader(vinegarlogo))

	return &Splash{
		logo:   logo,
		Theme:  th,
		Config: cfg,
		Window: app.NewWindow(
			app.Decorated(false),
			app.Size(width, height),
			app.MinSize(width, height),
			app.MaxSize(width, height),
			app.Title("Vinegar"),
		),
	}
}

func (ui *Splash) Run() error {
	var ops op.Ops
	var showLogButton widget.Clickable
	var exitButton widget.Clickable

	if !ui.Config.Enabled {
		return nil
	}

	for e := range ui.Events() {
		switch e := e.(type) {
		case system.DestroyEvent:
			if ui.closed {
				return nil
			} else if e.Err == nil {
				return ErrClosed
			} else {
				return e.Err
			}
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)
			paint.Fill(gtx.Ops, ui.Theme.Palette.Bg)

			if showLogButton.Clicked() {
				err := util.XDGOpen(ui.showLog).Start()
				if err != nil {
					return err
				}
			}

			if exitButton.Clicked() {
				ui.Perform(system.ActionClose)
			}

			layout.Center.Layout(gtx, func(gtx C) D {
				return layout.Flex{
					Axis:      layout.Vertical,
					Alignment: layout.Middle,
				}.Layout(gtx,
					layout.Rigid(widget.Image{Src: paint.NewImageOp(ui.logo)}.Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
					layout.Rigid(material.Label(ui.Theme, unit.Sp(16), ui.message).Layout),

					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Top:    unit.Dp(14),
							Bottom: unit.Dp(20),
							Right:  unit.Dp(25),
							Left:   unit.Dp(25),
						}.Layout(gtx, func(gtx C) D {
							pb := ProgressBar(ui.Theme, ui.progress)
							pb.TrackColor = rgb(ui.Config.Gray1)
							return pb.Layout(gtx)
						})
					}),

					layout.Rigid(func(gtx C) D {
						if ui.log == "" {
							return D{}
						}

						info := material.Body2(ui.Theme, ui.log)
						info.Color = ui.Theme.Palette.ContrastFg
						return info.Layout(gtx)
					}),

					layout.Rigid(func(gtx C) D {
						if ui.desc == "" {
							return D{}
						}

						info := material.Body2(ui.Theme, ui.desc)
						info.Color = ui.Theme.Palette.ContrastFg
						return info.Layout(gtx)
					}),

					layout.Rigid(func(gtx C) D {
						inset := layout.Inset{
							Top:   unit.Dp(16),
							Right: unit.Dp(6),
							Left:  unit.Dp(6),
						}

						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return inset.Layout(gtx, func(gtx C) D {
									btn := material.Button(ui.Theme, &exitButton, "Cancel")
									btn.Background = rgb(ui.Config.Red)
									btn.Color = ui.Theme.Palette.Fg
									btn.CornerRadius = 16
									return btn.Layout(gtx)
								})
							}),
							layout.Rigid(func(gtx C) D {
								if ui.showLog == "" {
									return D{}
								}

								return inset.Layout(gtx, func(gtx C) D {
									btn := material.Button(ui.Theme, &showLogButton, "Show Log")
									btn.Color = ui.Theme.Palette.Fg
									btn.CornerRadius = 16
									return btn.Layout(gtx)
								})
							}),
						)
					}),
				)
			})

			e.Frame(gtx.Ops)
		}
	}

	return nil
}
