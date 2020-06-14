package main

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/jesseduffield/fyne"
	fyneApp "github.com/jesseduffield/fyne/app"
	"github.com/jesseduffield/fyne/dialog"
	"github.com/jesseduffield/fyne/layout"
	"github.com/jesseduffield/fyne/theme"
	"github.com/jesseduffield/fyne/widget"
	"github.com/jesseduffield/horcrux/pkg/commands"
	"github.com/skratchdot/open-golang/open"
)

type app struct {
	fyneApp fyne.App
	window  fyne.Window
	refresh func()

	tabIndex int

	// for the create tab
	horcruxCount          int
	horcruxRequiredCount  int
	sourcePath            string
	createDestinationPath string

	// for the combine tab
	combineDestinationPath string
	horcruxPaths           []string
	horcruxValidationError string
	horcruxTotal           int
	horcruxThreshold       int
}

// attempt to obtain headers for them all and if you can't raise an error. Then append by path name.

func includeString(haystack []string, needle string) bool {
	for _, el := range haystack {
		if el == needle {
			return true
		}
	}
	return false
}

const maxHorcruxCount = 7

func newApp() *app {
	fApp := fyneApp.New()
	window := fApp.NewWindow("Horcrux")
	window.Resize(fyne.Size{Width: 700, Height: 480})
	window.SetFixedSize(true)

	return &app{
		fyneApp:              fApp,
		horcruxCount:         maxHorcruxCount,
		horcruxRequiredCount: maxHorcruxCount,
		window:               window,
	}
}

func numberOptions(limit int) []string {
	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = strconv.Itoa(i + 1)
	}
	return result
}

func (app *app) updateHorcruxes(paths []string) {
	horcruxes, err := commands.GetHorcruxes(paths)
	if err != nil {
		dialog.ShowError(err, app.window)
		return
	}

	if err := commands.ValidateHorcruxes(horcruxes); err != nil {
		app.horcruxValidationError = err.Error()
		app.combineDestinationPath = ""
		app.horcruxThreshold = 1 // dummy values
		app.horcruxTotal = 1
	} else {
		app.horcruxValidationError = ""
		firstHorcrux := horcruxes[0]
		originalFilename := firstHorcrux.GetHeader().OriginalFilename
		path := horcruxes[0].GetPath()
		app.combineDestinationPath = filepath.Join(filepath.Dir(path), originalFilename)
		app.horcruxThreshold = firstHorcrux.GetHeader().Threshold
		app.horcruxTotal = firstHorcrux.GetHeader().Total
	}

	app.horcruxPaths = paths
}

func (app *app) createTab() fyne.CanvasObject {
	horcruxCountRadio := widget.NewRadio(numberOptions(maxHorcruxCount), func(s string) {
		horcruxCount, err := strconv.Atoi(s)
		if err != nil {
			app.refresh()
			return
		}
		app.horcruxCount = horcruxCount
		if app.horcruxRequiredCount > horcruxCount {
			app.horcruxRequiredCount = horcruxCount
		}
		app.refresh()
	})
	horcruxCountRadio.Horizontal = true
	horcruxCountRadio.Selected = strconv.Itoa(app.horcruxCount)
	if app.sourcePath == "" {
		horcruxCountRadio.Disable()
	}

	requiredCountRadio := widget.NewRadio(numberOptions(app.horcruxCount), func(s string) {
		n, err := strconv.Atoi(s)
		if err != nil {
			app.refresh()
			return
		}
		app.horcruxRequiredCount = n
		app.refresh()
	})
	requiredCountRadio.Horizontal = true
	requiredCountRadio.Selected = strconv.Itoa(app.horcruxRequiredCount)
	if app.sourcePath == "" {
		requiredCountRadio.Disable()
	}

	createButton := widget.NewButton("Create horcruxes", func() {
		err := commands.Split(app.sourcePath, app.createDestinationPath, app.horcruxCount, app.horcruxRequiredCount)
		if err != nil {
			dialog.ShowError(err, app.window)
			return
		}
		if err := open.Start(app.createDestinationPath); err != nil {
			dialog.ShowError(err, app.window)
			return
		}
	})
	if app.sourcePath == "" {
		createButton.Disable()
	}

	return widget.NewVBox(
		widget.NewLabel("With Horcrux you can create horcruxes out of your files to be recombined again without requiring a password"),
		widget.NewVBox(
			widget.NewHBox(
				widget.NewButton("Select file", func() {
					fd := dialog.NewFileOpen(func(readers []fyne.URIReadCloser, err error) {
						if err != nil {
							dialog.ShowError(err, app.window)
							return
						}
						if len(readers) == 0 {
							return
						}
						if len(readers) > 1 {
							dialog.ShowError(errors.New("You must only select one file"), app.window)
							return
						}
						reader := readers[0]

						uri := reader.URI().String() // file:///Users/jesseduffield/tick.png
						u, err := url.Parse(uri)
						if err != nil {
							dialog.ShowError(err, app.window)
							return
						}

						stat, err := os.Stat(u.Path)
						if err != nil {
							dialog.ShowError(err, app.window)
							return
						}

						if stat.IsDir() {
							dialog.ShowError(errors.New("Must select a file, not a directory"), app.window)
							return
						}

						app.sourcePath = u.Path // /Users/jesseduffield/tick.png
						if app.createDestinationPath == "" {
							basename := filepath.Base(u.Path)
							basenameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))
							app.createDestinationPath = filepath.Join(filepath.Dir(u.Path), basenameWithoutExt+"_horcruxes")
						}

						app.refresh()
					}, app.window)
					fd.Show()
				}),
				widget.NewLabel(app.sourcePath),
			),
			widget.NewVBox(
				widget.NewLabel("Horcruxes to create"),
				horcruxCountRadio,
			),
			widget.NewVBox(
				widget.NewLabel("Horcruxes required to recreate original file"),
				requiredCountRadio,
			),
		),
		widget.NewVBox(
			widget.NewHBox(
				createButton,
			),
		),
		layout.NewSpacer(),
		app.quitButton(),
	)
}

func (app *app) combineTab() fyne.CanvasObject {
	horcruxPathWidgets := make([]fyne.CanvasObject, len(app.horcruxPaths))
	for i, path := range app.horcruxPaths {
		horcruxPathWidgets[i] = widget.NewHBox(
			widget.NewButton("Remove", func() {
				app.horcruxPaths = append(app.horcruxPaths[0:i], app.horcruxPaths[i+1:]...)
				app.updateHorcruxes(app.horcruxPaths)
				app.refresh()
			}),
			widget.NewLabel(path),
		)
	}

	horcruxPathsBox := widget.NewVBox(horcruxPathWidgets...)

	combineButton := widget.NewButton("Combine horcruxes", func() {
		// we can work out what the directory will be here. It probably doesn't matter where you save it. You could copy it somewhere else afterwards
		horcruxes, err := commands.GetHorcruxes(app.horcruxPaths)
		if err != nil {
			dialog.ShowError(err, app.window)
			return
		}

		if err := commands.ValidateHorcruxes(horcruxes); err != nil {
			app.horcruxValidationError = err.Error()
		}

		var combineHorcruxes func(bool)
		combineHorcruxes = func(overwrite bool) {
			err = commands.Bind(app.horcruxPaths, app.combineDestinationPath, overwrite)
			if err != nil {
				if err == os.ErrExist {
					dialog.ShowConfirm("File exists", fmt.Sprintf("A file already exists at path %s. Overwrite file?", app.combineDestinationPath), func(overwriteResponse bool) {
						if overwriteResponse {
							combineHorcruxes(true)
						}
					}, app.window)
				} else {
					dialog.ShowError(err, app.window)
				}
				return
			}

			if err := open.Start(filepath.Dir(app.combineDestinationPath)); err != nil {
				dialog.ShowError(err, app.window)
				return
			}
		}

		combineHorcruxes(false)

	})
	if len(app.horcruxPaths) == 0 || app.horcruxValidationError != "" || app.combineDestinationPath == "" {
		combineButton.Disable()
	}

	validationErrorWidget := widget.NewLabelWithStyle(app.horcruxValidationError, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	destinationText := ""
	if app.combineDestinationPath != "" {
		destinationText = fmt.Sprintf("Destination: %s", app.combineDestinationPath)
	}
	destinationWidget := widget.NewLabelWithStyle(destinationText, fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	return widget.NewVBox(
		widget.NewVBox(
			widget.NewHBox(
				widget.NewButton("Select horcruxes", func() {
					fd := dialog.NewFileOpen(func(readers []fyne.URIReadCloser, err error) {
						if err != nil {
							dialog.ShowError(err, app.window)
							return
						}
						if len(readers) == 0 {
							return
						}

						paths := []string{}
						paths = append(paths, app.horcruxPaths...)

						for _, reader := range readers {
							path := strings.TrimPrefix(reader.URI().String(), "file://")
							if includeString(paths, path) {
								continue
							}
							paths = append(paths, path)
						}

						app.updateHorcruxes(paths)

						app.refresh()
					}, app.window)
					fd.Show()
					fd.SetMultiSelect(true)
				}),
			),
			horcruxPathsBox,
			validationErrorWidget,
			destinationWidget,
			widget.NewHBox(combineButton),
		),
		layout.NewSpacer(),
		app.quitButton(),
	)
}

func (app *app) aboutTab() fyne.CanvasObject {
	return widget.NewVBox(
		widget.NewVBox(
			widget.NewLabel("Horcrux, created by Jesse Duffield"),
			widget.NewButton("Github page", func() {
				openUrlInBrowser("https://github.com/jesseduffield/horcrux-ui")
			}),
			widget.NewButton("Raise an issue", func() {
				openUrlInBrowser("https://github.com/jesseduffield/horcrux-ui/issues/new")
			}),
			widget.NewButton("Sponsor me", func() {
				openUrlInBrowser("https://github.com/sponsors/jesseduffield")
			}),
			widget.NewButton("Visit my website", func() {
				openUrlInBrowser("https://jesseduffield.com/")
			}),
		),
		layout.NewSpacer(),
		app.quitButton(),
	)
}
func (app *app) quitButton() fyne.CanvasObject {
	return widget.NewHBox(
		layout.NewSpacer(),
		widget.NewButton("Quit", func() {
			app.fyneApp.Quit()
		}),
	)
}

func main() {
	app := newApp()

	app.refresh = func() {
		tabs := widget.NewTabContainer(
			widget.NewTabItemWithIcon("Create Horcruxes", theme.ViewFullScreenIcon(), app.createTab()),
			widget.NewTabItemWithIcon("Combine Horcruxes", theme.ViewRestoreIcon(), app.combineTab()),
			widget.NewTabItemWithIcon("About", theme.InfoIcon(), app.aboutTab()),
		)

		tabs.SetTabLocation(widget.TabLocationLeading)
		tabs.SelectTabIndex(app.tabIndex)
		tabs.OnChanged = func(tab *widget.TabItem, index int) {
			app.tabIndex = index
		}

		app.window.SetContent(
			tabs,
		)
	}

	app.refresh()

	app.window.ShowAndRun()
}

func openUrlInBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}

}
