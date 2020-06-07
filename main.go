package main

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jesseduffield/fyne"
	fyneApp "github.com/jesseduffield/fyne/app"
	"github.com/jesseduffield/fyne/cmd/fyne_demo/screens"
	"github.com/jesseduffield/fyne/dialog"
	"github.com/jesseduffield/fyne/layout"
	"github.com/jesseduffield/fyne/theme"
	"github.com/jesseduffield/fyne/widget"
	"github.com/jesseduffield/horcrux/pkg/commands"
	"github.com/skratchdot/open-golang/open"
)

type app struct {
	fyneApp fyne.App

	horcruxCount         int
	horcruxRequiredCount int
	directoryPath        string
	sourcePath           string
	destinationPath      string
	horcruxPaths         []string
}

const maxHorcruxCount = 7

func newApp() *app {
	return &app{
		fyneApp:              fyneApp.New(),
		horcruxCount:         maxHorcruxCount,
		horcruxRequiredCount: maxHorcruxCount,
	}
}

func numberOptions(limit int) []string {
	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = strconv.Itoa(i + 1)
	}
	return result
}

func main() {
	app := newApp()
	var refresh func()

	w := app.fyneApp.NewWindow("Horcrux")

	refresh = func() {
		horcruxCountRadio := widget.NewRadio(numberOptions(maxHorcruxCount), func(s string) {
			horcruxCount, err := strconv.Atoi(s)
			if err != nil {
				refresh()
				return
			}
			app.horcruxCount = horcruxCount
			if app.horcruxRequiredCount > horcruxCount {
				app.horcruxRequiredCount = horcruxCount
			}
			refresh()
		})
		horcruxCountRadio.Horizontal = true
		horcruxCountRadio.Selected = strconv.Itoa(app.horcruxCount)
		if app.sourcePath == "" {
			horcruxCountRadio.Disable()
		}

		requiredCountRadio := widget.NewRadio(numberOptions(app.horcruxCount), func(s string) {
			n, err := strconv.Atoi(s)
			if err != nil {
				refresh()
				return
			}
			app.horcruxRequiredCount = n
			refresh()
		})
		requiredCountRadio.Horizontal = true
		requiredCountRadio.Selected = strconv.Itoa(app.horcruxRequiredCount)
		if app.sourcePath == "" {
			requiredCountRadio.Disable()
		}

		goButton := widget.NewButton("Create horcruxes", func() {
			err := commands.Split(app.sourcePath, app.destinationPath, app.horcruxCount, app.horcruxRequiredCount)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if err := open.Start(app.destinationPath); err != nil {
				dialog.ShowError(err, w)
				return
			}

		})
		if app.sourcePath == "" {
			goButton.Disable()
		}

		createTab := widget.NewVBox(
			widget.NewLabel("With Horcrux you can create horcruxes out of your files to be recombined again without requiring a password"),
			widget.NewVBox(
				layout.NewSpacer(),
				widget.NewHBox(
					widget.NewButton("Select file", func() {
						fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
							if err == nil && reader == nil {
								return
							}
							if err != nil {
								dialog.ShowError(err, w)
								return
							}

							uri := reader.URI().String() // file:///Users/jesseduffield/tick.png
							u, err := url.Parse(uri)
							if err != nil {
								dialog.ShowError(err, w)
								return
							}

							stat, err := os.Stat(u.Path)
							if err != nil {
								dialog.ShowError(err, w)
								return
							}

							if stat.IsDir() {
								dialog.ShowError(errors.New("Must select a file, not a directory"), w)
								return
							}

							app.sourcePath = u.Path // /Users/jesseduffield/tick.png
							if app.destinationPath == "" {
								basename := filepath.Base(u.Path)
								basenameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))
								app.destinationPath = filepath.Join(filepath.Dir(u.Path), basenameWithoutExt+"_horcruxes")
							}

							refresh()
						}, w)
						fd.Show()
					}),
					widget.NewLabel(app.sourcePath),
				),
				layout.NewSpacer(),
				widget.NewVBox(
					widget.NewLabel("Horcruxes to create"),
					horcruxCountRadio,
				),
				widget.NewVBox(
					widget.NewLabel("Horcruxes required to recreate original file"),
					requiredCountRadio,
				),
				layout.NewSpacer(),
			),
			widget.NewVBox(
				widget.NewHBox(
					goButton,
				),
				layout.NewSpacer(),
			),
		)

		combineTab := widget.NewVBox(
			widget.NewVBox(
				layout.NewSpacer(),
				widget.NewHBox(
					widget.NewButton("Select directory", func() {
						fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
							dialog.NewError(errors.New("Combining horcruxes is not yet implemented"), w)
						}, w)
						fd.Show()
					}),
					widget.NewLabel(app.sourcePath),
				),
			))

		tabs := widget.NewTabContainer(
			widget.NewTabItemWithIcon("Create Horcruxes", theme.ViewFullScreenIcon(), createTab),
			widget.NewTabItemWithIcon("Combine Horcruxes", theme.ViewRestoreIcon(), combineTab),
			widget.NewTabItemWithIcon("About", theme.InfoIcon(), screens.WidgetScreen()))

		tabs.SetTabLocation(widget.TabLocationLeading)
		tabs.SelectTabIndex(0)

		w.SetContent(tabs)
	}

	refresh()

	w.ShowAndRun()
}
