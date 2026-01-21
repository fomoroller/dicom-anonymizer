package gui

import (
	"os/exec"
	"runtime"

	dcm "dicom-anonymizer/internal/dicom"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	AppTitle  = "DICOM Anonymization Tool"
	AppWidth  = 650
	AppHeight = 600
)

// App represents the GUI application
type App struct {
	fyneApp    fyne.App
	mainWindow fyne.Window
	wizard     *Wizard
	steps      *StepBuilder

	// dcmtk status indicator
	dcmtkStatusCircle *canvas.Circle
	dcmtkStatusLabel  *widget.Label
}

// NewApp creates a new GUI application
func NewApp() *App {
	a := app.New()
	a.SetIcon(resourceIconPng)
	a.Settings().SetTheme(&ModernTheme{})

	return &App{
		fyneApp: a,
	}
}

// Run starts the GUI application
func (a *App) Run() {
	a.mainWindow = a.fyneApp.NewWindow(AppTitle)
	a.mainWindow.Resize(fyne.NewSize(AppWidth, AppHeight))
	// Position window near top of screen (50 pixels from top)
	a.mainWindow.CenterOnScreen()

	// Create wizard
	a.wizard = NewWizard(a.mainWindow)

	// Create and set dcmtk status indicator
	dcmtkStatus := a.createDcmtkStatusIndicator()
	a.wizard.SetStatusIndicator(dcmtkStatus)

	// Set initial dcmtk status (this will enable/disable the wizard)
	a.wizard.SetDcmtkInstalled(dcm.CheckDcmtkInstalled())

	// Create step builder
	a.steps = NewStepBuilder(a.mainWindow, a.wizard)

	// Build step content
	a.wizard.SetStepContent(StepInput, a.steps.BuildStep1())
	a.wizard.SetStepContent(StepSettings, a.steps.BuildStep2())
	a.wizard.SetStepContent(StepPreview, a.steps.BuildStep3())
	a.wizard.SetStepContent(StepProcess, a.steps.BuildStep4())

	// Set validation callback
	a.wizard.SetCanProceed(func(step WizardStep) bool {
		switch step {
		case StepInput:
			return a.steps.ValidateStep1()
		case StepSettings:
			return a.steps.ValidateStep2()
		case StepPreview:
			return a.steps.dryRunComplete
		case StepProcess:
			// On process step, "Done" closes the app
			if !a.steps.IsProcessing() {
				a.mainWindow.Close()
			}
			return false
		}
		return true
	})

	// Set step change callback
	a.wizard.SetOnStepChange(func(step WizardStep) {
		switch step {
		case StepPreview:
			// Auto-run dry run when entering preview step
			a.steps.RunDryRun()
		case StepProcess:
			// Auto-run processing when entering process step
			a.steps.RunProcess()
		}
	})

	// Set dcmtk warning callback
	a.wizard.SetOnDcmtkWarning(func(proceed func()) {
		dialog.ShowConfirm("dcmtk Not Installed",
			"The dcmtk library is not installed. Some JPEG-LS compressed DICOM files may fail to process.\n\nDo you want to continue anyway?",
			func(confirmed bool) {
				if confirmed {
					proceed()
				}
			}, a.mainWindow)
	})

	// Build and set wizard UI
	content := a.wizard.Build()
	a.mainWindow.SetContent(content)

	a.maybePromptDcmtkInstall()

	// Confirm before closing if processing
	a.mainWindow.SetCloseIntercept(func() {
		if a.steps.IsProcessing() {
			dialog.ShowConfirm("Confirm Exit",
				"Processing is in progress. Are you sure you want to exit?",
				func(confirm bool) {
					if confirm {
						a.mainWindow.Close()
					}
				}, a.mainWindow)
		} else {
			a.mainWindow.Close()
		}
	})

	a.mainWindow.ShowAndRun()
}

func (a *App) maybePromptDcmtkInstall() {
	// Always check if dcmtk is installed
	if dcm.CheckDcmtkInstalled() {
		return
	}

	// Check if this is the first run (never prompted before)
	prefs := a.fyneApp.Preferences()
	firstRun := !prefs.BoolWithFallback("dcmtk_prompted", false)

	if firstRun {
		// Mark as prompted so we don't show automatically on every startup
		prefs.SetBool("dcmtk_prompted", true)
		// Show the install dialog automatically on first run
		a.showDcmtkInstallDialog()
	}
	// On subsequent runs, users can click the status indicator to see the dialog
}

func getDcmtkInstallCommand() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install dcmtk"
	case "linux":
		return "sudo apt-get update && sudo apt-get install -y dcmtk"
	default:
		return ""
	}
}

func getDcmtkInstallHint() string {
	cmd := getDcmtkInstallCommand()
	if cmd == "" {
		return "Install dcmtk using your system package manager."
	}
	return cmd
}

// createDcmtkStatusIndicator creates a clickable dcmtk status indicator with a colored circle
func (a *App) createDcmtkStatusIndicator() fyne.CanvasObject {
	// Create status circle (green if installed, red if not)
	a.dcmtkStatusCircle = canvas.NewCircle(ColorStatusRed)
	a.dcmtkStatusCircle.StrokeWidth = 0

	// Create label
	a.dcmtkStatusLabel = widget.NewLabel("dcmtk")
	a.dcmtkStatusLabel.TextStyle = fyne.TextStyle{Bold: false}

	// Update initial status (without wizard update since wizard doesn't exist yet)
	if dcm.CheckDcmtkInstalled() {
		a.dcmtkStatusCircle.FillColor = ColorStatusGreen
		a.dcmtkStatusLabel.SetText("dcmtk: OK")
	} else {
		a.dcmtkStatusCircle.FillColor = ColorStatusRed
		a.dcmtkStatusLabel.SetText("dcmtk: Missing")
	}

	// Create a button that shows the status and is clickable
	statusBtn := widget.NewButton("", func() {
		a.showDcmtkInstallDialog()
	})
	statusBtn.Importance = widget.LowImportance

	// Use custom layout to vertically center the circle with the label
	statusContent := container.New(&dcmtkStatusLayout{}, a.dcmtkStatusCircle, a.dcmtkStatusLabel)

	// Use a stack layout to overlay the button
	clickableStatus := container.NewStack(
		statusBtn,
		statusContent,
	)

	return clickableStatus
}

// dcmtkStatusLayout is a custom layout that vertically centers a circle with a label
type dcmtkStatusLayout struct{}

func (l *dcmtkStatusLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	circleSize := float32(10)
	labelSize := objects[1].MinSize()
	return fyne.NewSize(circleSize+8+labelSize.Width, labelSize.Height)
}

func (l *dcmtkStatusLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	circle := objects[0]
	label := objects[1]

	circleSize := float32(10)
	labelSize := label.MinSize()

	// Center circle vertically with the label
	circleY := (size.Height - circleSize) / 2
	circle.Resize(fyne.NewSize(circleSize, circleSize))
	circle.Move(fyne.NewPos(4, circleY))

	// Position label after circle with some spacing
	label.Resize(labelSize)
	label.Move(fyne.NewPos(circleSize+12, (size.Height-labelSize.Height)/2))
}

// updateDcmtkStatus updates the dcmtk status indicator and wizard state
func (a *App) updateDcmtkStatus() {
	installed := dcm.CheckDcmtkInstalled()
	if installed {
		a.dcmtkStatusCircle.FillColor = ColorStatusGreen
		a.dcmtkStatusLabel.SetText("dcmtk: OK")
	} else {
		a.dcmtkStatusCircle.FillColor = ColorStatusRed
		a.dcmtkStatusLabel.SetText("dcmtk: Missing")
	}
	a.dcmtkStatusCircle.Refresh()
	a.dcmtkStatusLabel.Refresh()

	// Update wizard state based on dcmtk status
	if a.wizard != nil {
		a.wizard.SetDcmtkInstalled(installed)
	}
}

// IsDcmtkInstalled returns whether dcmtk is currently installed
func (a *App) IsDcmtkInstalled() bool {
	return dcm.CheckDcmtkInstalled()
}

// showDcmtkInstallDialog shows the dcmtk installation dialog
func (a *App) showDcmtkInstallDialog() {
	installed := dcm.CheckDcmtkInstalled()

	var status *widget.Label
	if installed {
		status = widget.NewLabel("dcmtk is installed and ready to use.")
	} else {
		status = widget.NewLabel("dcmtk is NOT installed.\n\nThis tool requires dcmtk to process JPEG-LS compressed DICOM files. Please install it to continue.")
	}
	status.Wrapping = fyne.TextWrapWord

	commandLabel := widget.NewLabel(getDcmtkInstallHint())
	commandLabel.Wrapping = fyne.TextWrapWord

	var installBtn *widget.Button
	installBtn = widget.NewButton("Install dcmtk", func() {
		installBtn.Disable()
		status.SetText("Installing dcmtk. This may take a minute...")
		status.Refresh()

		command := getDcmtkInstallCommand()
		if command == "" {
			status.SetText("Automatic install is not available on this OS. Use the command below.")
			status.Refresh()
			installBtn.Enable()
			return
		}

		go func() {
			cmd := exec.Command("bash", "-lc", command)
			output, err := cmd.CombinedOutput()
			if err != nil {
				status.SetText("Install failed. See details in the console output.")
				status.Refresh()
				dialog.ShowError(err, a.mainWindow)
				installBtn.Enable()
				return
			}
			if len(output) > 0 {
				dialog.ShowInformation("dcmtk install", string(output), a.mainWindow)
			}
			status.SetText("dcmtk installed successfully! You can now use the app.")
			status.Refresh()
			a.updateDcmtkStatus()
			installBtn.Hide()
		}()
	})

	if installed {
		installBtn.Hide()
	}

	content := container.NewVBox(
		status,
		widget.NewSeparator(),
		widget.NewLabel("Manual installation command:"),
		commandLabel,
		installBtn,
	)

	title := "dcmtk Status"
	if !installed {
		title = "dcmtk Required"
	}

	d := dialog.NewCustom(title, "Close", content, a.mainWindow)
	d.Resize(fyne.NewSize(400, 250))
	d.Show()
}
