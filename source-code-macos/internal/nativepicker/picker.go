package nativepicker

import (
	"github.com/ncruces/zenity"
)

type Picker interface {
	Game() (string, error)
	Mod() (string, error)
	SaveMod(string) (string, error)
}
type System struct{}

func (System) Game() (string, error) {
	return zenity.SelectFile(zenity.Title("Choose Geometry Dash"), zenity.FileFilters{{Name: "Geometry Dash", Patterns: []string{"*.app", "*.exe"}}, {Name: "All files", Patterns: []string{"*"}}})
}
func (System) Mod() (string, error) {
	return zenity.SelectFile(zenity.Title("Choose a .dash mod"), zenity.FileFilters{{Name: "Dash mod", Patterns: []string{"*.dash", "*.json"}}})
}
func (System) SaveMod(name string) (string, error) {
	return zenity.SelectFileSave(zenity.Title("Save mod"), zenity.Filename(name), zenity.ConfirmOverwrite(), zenity.FileFilters{{Name: "Dash mod", Patterns: []string{"*.dash"}}})
}
func Cancelled(err error) bool         { return err == zenity.ErrCanceled }
func ShowError(name string, err error) { _ = zenity.Error(err.Error(), zenity.Title(name)) }
