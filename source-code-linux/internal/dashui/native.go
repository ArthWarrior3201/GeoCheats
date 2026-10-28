//go:build !darwin

package dashui

import (
	"embed"
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	loader "geocheats/internal/app"
	"geocheats/internal/core"
	"geocheats/internal/nativedesktop"
	"geocheats/internal/nativepicker"
	"image/color"
	"os"
	"path/filepath"
	"strings"
)

//go:embed dash.png
var nativeAssets embed.FS

func Run() {
	a := fyneapp.NewWithID("app.geocheats.dash.native")
	logo, _ := nativeAssets.ReadFile("dash.png")
	nativedesktop.Setup(a, logo, "Dash")
	w := a.NewWindow("Dash")
	w.Resize(fyne.NewSize(530, 590))
	picker := nativepicker.System{}
	game := ""
	var image core.Image
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	name := widget.NewEntry()
	name.SetPlaceHolder("My mod")
	id := widget.NewEntry()
	id.SetPlaceHolder("creator.my-mod")
	address := widget.NewEntry()
	address.SetPlaceHolder("0x1234")
	expected := widget.NewEntry()
	expected.SetPlaceHolder("55")
	replacement := widget.NewEntry()
	replacement.SetPlaceHolder("C3")
	form := widget.NewForm(widget.NewFormItem("Name", name), widget.NewFormItem("Mod ID", id), widget.NewFormItem("Address (RVA)", address), widget.NewFormItem("Expected bytes", expected), widget.NewFormItem("Replacement bytes", replacement))
	var advanced *core.Package
	linkedLabel := widget.NewLabel("Geometry Dash linked")
	var prompt, body fyne.CanvasObject
	accept := func(path string) error {
		p, e := loader.ResolveExecutable(path)
		if e != nil {
			return e
		}
		im, e := core.Inspect(p)
		if e != nil {
			return e
		}
		game = p
		image = im
		a.Preferences().SetString("game", p)
		advanced = nil
		prompt.Hide()
		body.Show()
		status.SetText("")
		return nil
	}
	choose := func() {
		go func() {
			path, e := picker.Game()
			if nativepicker.Cancelled(e) {
				return
			}
			fyne.Do(func() {
				if e == nil {
					e = accept(path)
				}
				if e != nil {
					dialog.ShowError(e, w)
				}
			})
		}()
	}
	prompt = container.NewCenter(container.NewVBox(widget.NewLabel("Link Geometry Dash"), widget.NewButton("Choose file…", choose)))
	build := func() (core.Package, error) {
		p := core.Package{Schema: 2, ID: id.Text, Name: name.Text, Target: image.SHA256, Format: image.Format, Arch: image.Arch, Kind: "patch", Status: "Unverified gameplay", Patches: []core.Patch{{RVA: address.Text, Expected: expected.Text, Replacement: replacement.Text}}}
		if advanced != nil {
			p = *advanced
			if p.Target == "AUTO" {
				p.Target = image.SHA256
			}
			if p.Format == "AUTO" {
				p.Format = image.Format
			}
			if p.Arch == "AUTO" {
				p.Arch = image.Arch
			}
		}
		if e := core.VerifyFile(p, game); e != nil {
			return p, e
		}
		return p, nil
	}
	save := widget.NewButton("Create .dash", func() {
		p, e := build()
		if e != nil {
			dialog.ShowError(e, w)
			return
		}
		go func() {
			path, e := picker.SaveMod(p.ID + ".dash")
			if nativepicker.Cancelled(e) {
				return
			}
			if e == nil && !strings.EqualFold(filepath.Ext(path), ".dash") {
				e = fmt.Errorf("save the file with a .dash extension")
			}
			if e == nil {
				var data []byte
				data, e = json.MarshalIndent(p, "", "  ")
				if e == nil {
					e = os.WriteFile(path, append(data, '\n'), 0644)
				}
			}
			fyne.Do(func() {
				if e != nil {
					dialog.ShowError(e, w)
				} else {
					status.SetText("Saved " + filepath.Base(path))
				}
			})
		}()
	})
	save.Importance = widget.HighImportance
	importRecipe := widget.NewButton("Open .dash…", func() {
		go func() {
			path, e := picker.Mod()
			if nativepicker.Cancelled(e) {
				return
			}
			var p core.Package
			if e == nil {
				var data []byte
				data, e = os.ReadFile(path)
				if e == nil {
					e = json.Unmarshal(data, &p)
				}
			}
			fyne.Do(func() {
				if e != nil {
					dialog.ShowError(e, w)
					return
				}
				name.SetText(p.Name)
				id.SetText(p.ID)
				if len(p.Patches) > 0 {
					address.SetText(p.Patches[0].RVA)
					expected.SetText(p.Patches[0].Expected)
					replacement.SetText(p.Patches[0].Replacement)
				}
				advanced = &p
				status.SetText("Recipe opened.")
			})
		}()
	})
	codeButton := widget.NewButton("ModCode…", func() {
		code := widget.NewMultiLineEntry()
		code.SetText("modcode 1\nname \"My mod\"\nid \"author.my-mod\"\n# Replace these bytes and address with verified values\npatch 0x1234 \"55\" -> \"C3\"\n")
		code.SetMinRowsVisible(12)
		d := dialog.NewCustomConfirm("ModCode", "Use code", "Cancel", code, func(ok bool) {
			if !ok {
				return
			}
			p, err := core.ParseModCode(code.Text, image)
			if err == nil {
				err = core.VerifyFile(p, game)
			}
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			name.SetText(p.Name)
			id.SetText(p.ID)
			advanced = &p
			status.SetText("ModCode validated. Create the .dash file.")
		}, w)
		d.Resize(fyne.NewSize(520, 400))
		d.Show()
	})
	advancedButton := widget.NewButton("Advanced…", func() {
		p, _ := build()
		if advanced != nil {
			p = *advanced
		}
		data, _ := json.MarshalIndent(p, "", "  ")
		text := widget.NewMultiLineEntry()
		text.SetText(string(data))
		text.SetMinRowsVisible(15)
		d := dialog.NewCustomConfirm("Recipe JSON", "Use recipe", "Cancel", text, func(ok bool) {
			if !ok {
				return
			}
			var p core.Package
			if e := json.Unmarshal([]byte(text.Text), &p); e != nil {
				dialog.ShowError(e, w)
				return
			}
			name.SetText(p.Name)
			id.SetText(p.ID)
			advanced = &p
		}, w)
		d.Resize(fyne.NewSize(500, 460))
		d.Show()
	})
	name.OnChanged = func(text string) {
		if advanced != nil {
			advanced.Name = text
		}
	}
	id.OnChanged = func(text string) {
		if advanced != nil {
			advanced.ID = text
		}
	}
	for _, entry := range []*widget.Entry{address, expected, replacement} {
		entry.OnChanged = func(string) { advanced = nil }
	}
	// Loading a complete recipe sets fields first, then preserves the advanced data.
	body = container.NewBorder(linkedLabel, container.NewVBox(status, container.NewHBox(save, importRecipe, codeButton, advancedButton)), nil, nil, form)
	body.Hide()
	icon := canvas.NewImageFromResource(fyne.NewStaticResource("Dash.png", logo))
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(44, 44))
	header := container.NewHBox(icon, widget.NewLabelWithStyle("Dash", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	rainbow := container.NewGridWithColumns(6)
	for _, c := range []color.NRGBA{{R: 255, G: 100, B: 135, A: 255}, {R: 255, G: 170, B: 90, A: 255}, {R: 255, G: 218, B: 100, A: 255}, {R: 100, G: 215, B: 165, A: 255}, {R: 90, G: 165, B: 250, A: 255}, {R: 195, G: 120, B: 235, A: 255}} {
		bar := canvas.NewRectangle(c)
		bar.SetMinSize(fyne.NewSize(1, 5))
		rainbow.Add(bar)
	}
	w.SetContent(container.NewPadded(container.NewBorder(container.NewVBox(header, rainbow), nil, nil, nil, container.NewStack(prompt, body))))
	w.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("App", fyne.NewMenuItem("Change game…", choose), fyne.NewMenuItem("Quit", a.Quit))))
	if previous := a.Preferences().String("game"); previous != "" {
		_ = accept(previous)
	}
	if game == "" {
		for _, path := range loader.InstalledGames() {
			if accept(path) == nil {
				break
			}
		}
	}
	w.Show()
	if report := nativedesktop.ReportPath(); report != "" {
		nativedesktop.Check(a, w, report, "Dash", func() map[string]any {
			return map[string]any{"linked": game != "", "choosePromptVisible": prompt.Visible(), "nativeChooser": true}
		})
	}
	a.Run()
}
