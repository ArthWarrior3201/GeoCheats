//go:build !darwin

package app

import (
	"embed"
	"fmt"
	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"geocheats/internal/nativedesktop"
	"geocheats/internal/nativepicker"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

//go:embed geocheats.png
var nativeAssets embed.FS

type flightButton struct {
	widget.Button
	direction int
	move      func(int)
}

func newFlightButton(label string, direction int, move func(int)) *flightButton {
	b := &flightButton{direction: direction, move: move}
	b.Text = label
	b.ExtendBaseWidget(b)
	return b
}
func (b *flightButton) MouseDown(event *desktop.MouseEvent) {
	if b.Disabled() {
		return
	}
	b.move(b.direction)
}
func (b *flightButton) MouseUp(event *desktop.MouseEvent) { b.move(0) }
func (b *flightButton) MouseOut()                         { b.Button.MouseOut(); b.move(0) }

type controls struct {
	root           *fyne.Container
	prompt, linked fyne.CanvasObject
	status         *widget.Label
	rows           *fyne.Container
	checks         map[string]*widget.Check
	signature      string
	service        *Service
	window         fyne.Window
	picker         nativepicker.Picker
}

func buildControls(w fyne.Window, s *Service, picker nativepicker.Picker) *controls {
	v := &controls{window: w, service: s, picker: picker, checks: map[string]*widget.Check{}}
	logo, _ := nativeAssets.ReadFile("geocheats.png")
	image := canvas.NewImageFromResource(fyne.NewStaticResource("GeoCheats.png", logo))
	image.FillMode = canvas.ImageFillContain
	image.SetMinSize(fyne.NewSize(44, 44))
	title := widget.NewLabelWithStyle("GeoCheats Mod Controller", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header := container.NewHBox(image, title, layout.NewSpacer(), widget.NewLabel("☁  ☁"))
	choose := widget.NewButton("Choose file…", v.chooseGame)
	v.prompt = container.NewCenter(container.NewVBox(widget.NewLabel("Link Geometry Dash"), choose))
	game := widget.NewLabel("Game Linked Successfully")
	launch := widget.NewButton("Launch game", func() { v.showError(s.Launch()) })
	importMod := widget.NewButton("Import mod…", func() {
		go func() {
			p, e := picker.Mod()
			if nativepicker.Cancelled(e) {
				return
			}
			if e == nil {
				var b []byte
				b, e = os.ReadFile(p)
				if e == nil {
					e = s.Import(b)
				}
			}
			fyne.Do(func() { v.showError(e); v.render() })
		}()
	})
	v.rows = container.NewVBox()
	v.status = widget.NewLabel("")
	v.status.Wrapping = fyne.TextWrapWord
	v.linked = container.NewBorder(container.NewHBox(game, layout.NewSpacer(), launch), container.NewVBox(v.status, container.NewHBox(importMod)), nil, nil, container.NewVScroll(v.rows))
	content := container.NewStack(v.prompt, v.linked)
	v.root = container.NewPadded(container.NewBorder(header, nil, nil, nil, content))
	v.render()
	return v
}

func (v *controls) showError(err error) {
	if err != nil {
		dialog.ShowInformation("Error", err.Error(), v.window)
	}
}
func (v *controls) chooseGame() {
	go func() {
		p, e := v.picker.Game()
		if nativepicker.Cancelled(e) {
			return
		}
		if e == nil {
			e = v.service.Link(p)
		}
		fyne.Do(func() { v.showError(e); v.render() })
	}()
}
func (v *controls) render() {
	state := v.service.Snapshot()
	if !state.Linked {
		v.prompt.Show()
		v.linked.Hide()
		return
	}
	v.prompt.Hide()
	v.linked.Show()
	v.status.SetText(state.Message)
	var signature strings.Builder
	for _, m := range state.Mods {
		fmt.Fprintf(&signature, "%s:%t:%t:%t;", m.Package.ID, m.Enabled, m.Active, m.Compatible)
	}
	if signature.String() == v.signature {
		return
	}
	v.signature = signature.String()
	v.rows.RemoveAll()
	v.checks = map[string]*widget.Check{}
	for _, mod := range state.Mods {
		m := mod
		check := widget.NewCheck(m.Package.Name, nil)
		check.SetChecked(m.Enabled)
		check.OnChanged = func(on bool) {
			if err := v.service.Toggle(m.Package.ID, on); err != nil {
				v.signature = ""
				v.showError(err)
			}
			v.render()
		}
		if !m.Compatible && !m.Enabled {
			check.Disable()
		}
		v.checks[m.Package.ID] = check
		status := ""
		if !m.Compatible {
			status = "Unavailable for this game build"
		} else if m.Enabled {
			if m.Active {
				status = "Active"
			} else {
				status = "Waiting for game"
			}
		}
		row := container.NewHBox(check, layout.NewSpacer(), widget.NewLabel(status))
		if m.Package.Kind == "hotkey" {
			b := widget.NewButton("Complete", func() { v.showError(v.service.Trigger()) })
			if !m.Active {
				b.Disable()
			}
			row.Add(b)
		}
		v.rows.Add(row)
		if m.Package.Effect == "fly" {
			move := func(d int) { _ = v.service.Flight(d) }
			up := newFlightButton("↑", 1, move)
			down := newFlightButton("↓", -1, move)
			if !m.Active {
				up.Disable()
				down.Disable()
			}
			row.Add(up)
			row.Add(down)
		}
	}
}

func Run() {
	report := nativedesktop.ReportPath()
	root := dataRoot()
	if report != "" {
		var e error
		root, e = os.MkdirTemp("", "geocheats-native-check-")
		if e != nil {
			fmt.Println(e)
			return
		}
		defer os.RemoveAll(root)
	}
	s, e := newService(root, true)
	if e != nil {
		if e != errAlreadyRunning {
			nativepicker.ShowError("GeoCheats", e)
		}
		return
	}
	defer s.Close()
	s.Start()
	a := fyneapp.NewWithID("app.geocheats.native")
	logo, _ := nativeAssets.ReadFile("geocheats.png")
	nativedesktop.Setup(a, logo, "GeoCheats")
	w := a.NewWindow("GeoCheats Mod Controller")
	w.Resize(fyne.NewSize(540, 540))
	ui := buildControls(w, s, nativepicker.System{})
	w.SetContent(ui.root)
	show := func() { w.Show(); w.RequestFocus() }
	quit := func() {
		if e := s.action("disconnect", request{}); e != nil {
			ui.showError(e)
			return
		}
		_ = s.Close()
		a.Quit()
	}
	change := fyne.NewMenuItem("Change game…", ui.chooseGame)
	login := fyne.NewMenuItem("Start at login", nil)
	login.Checked = s.Snapshot().LoginStart
	login.Action = func() { ui.showError(s.Login(!login.Checked)); login.Checked = s.Snapshot().LoginStart }
	w.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("App", change, login, fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Quit", quit))))
	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayMenu(fyne.NewMenu("GeoCheats", fyne.NewMenuItem("Open GeoCheats", show), fyne.NewMenuItem("Quit", quit)))
	}
	w.SetCloseIntercept(func() { _ = s.Flight(0); w.Hide() })
	if keys, ok := w.Canvas().(desktop.Canvas); ok {
		held := map[fyne.KeyName]bool{}
		keys.SetOnKeyDown(func(event *fyne.KeyEvent) {
			if held[event.Name] {
				return
			}
			held[event.Name] = true
			if held[fyne.KeyC] && held[fyne.KeyG] && held[fyne.KeyD] {
				ui.showError(s.Trigger())
			}
		})
		keys.SetOnKeyUp(func(event *fyne.KeyEvent) { delete(held, event.Name) })
		a.Lifecycle().SetOnExitedForeground(func() { fyne.Do(func() { clear(held); _ = s.Flight(0) }) })
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-s.wake:
				fyne.Do(show)
			case <-ticker.C:
				fyne.Do(ui.render)
			}
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		select {
		case <-done:
			return
		case <-signals:
			fyne.Do(quit)
		}
	}()
	background := len(os.Args) > 1 && os.Args[1] == "--background"
	if !background || report != "" {
		w.Show()
	}
	if report != "" {
		nativedesktop.Check(a, w, report, "GeoCheats", func() map[string]any {
			return map[string]any{"linked": s.Snapshot().Linked, "choosePromptVisible": ui.prompt.Visible(), "nativeChooser": true, "config": filepath.Base(root)}
		})
	}
	a.Run()
}
