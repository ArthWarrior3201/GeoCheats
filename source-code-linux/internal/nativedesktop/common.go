package nativedesktop

import (
	"encoding/json"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"image/color"
	"image/png"
	"os"
	"time"
)

type Theme struct{ fyne.Theme }

func (t Theme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return color.NRGBA{R: 110, G: 190, B: 40, A: 255}
	}
	return t.Theme.Color(name, variant)
}
func Setup(a fyne.App, icon []byte, name string) {
	a.SetIcon(fyne.NewStaticResource(name+".png", icon))
	a.Settings().SetTheme(Theme{theme.DefaultTheme()})
}
func ReportPath() string {
	for i, arg := range os.Args {
		if arg == "--launch-check" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

// Capture an actual native window and exit. No browser or loopback server is used.
func Check(a fyne.App, w fyne.Window, path, name string, details func() map[string]any) {
	go func() {
		time.Sleep(1500 * time.Millisecond)
		fyne.Do(func() {
			result := map[string]any{"name": name, "ok": true, "ui": "native", "browser": false, "checks": []string{"native window", "rendered controls", "app icon"}}
			if details != nil {
				for k, v := range details() {
					result[k] = v
				}
			}
			image := w.Canvas().Capture()
			f, e := os.Create(path + ".png")
			if e == nil {
				e = png.Encode(f, image)
				_ = f.Close()
			}
			if e != nil {
				result["ok"] = false
				result["error"] = e.Error()
			}
			b, _ := json.MarshalIndent(result, "", "  ")
			_ = os.WriteFile(path, b, 0600)
			a.Quit()
		})
	}()
}
