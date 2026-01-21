package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// WizardStep represents a step in the wizard
type WizardStep int

const (
	StepInput WizardStep = iota
	StepSettings
	StepPreview
	StepProcess
)

// StepInfo contains the display information for a wizard step
type StepInfo struct {
	Number int
	Title  string
}

var stepInfos = []StepInfo{
	{1, "Input"},
	{2, "Settings"},
	{3, "Preview"},
	{4, "Process"},
}

// Wizard manages the wizard flow and UI
type Wizard struct {
	window      fyne.Window
	currentStep WizardStep

	// Step content containers
	stepContents map[WizardStep]fyne.CanvasObject

	// Navigation buttons
	backButton *widget.Button
	nextButton *widget.Button

	// Step indicator circles
	stepIndicators []*canvas.Circle
	stepLabels     []*canvas.Text

	// Main content container
	contentContainer *fyne.Container
	stepIndicator    fyne.CanvasObject

	// Status indicator (optional, shown in header)
	statusIndicator fyne.CanvasObject

	// dcmtk installation status
	dcmtkInstalled bool

	// Overlay for blocking UI when dcmtk is not installed
	blockingOverlay fyne.CanvasObject

	// Callbacks
	onStepChange    func(WizardStep)
	canProceed      func(WizardStep) bool
	onDcmtkWarning  func(proceed func()) // Called when proceeding without dcmtk; proceed() continues navigation
}

// NewWizard creates a new wizard instance
func NewWizard(window fyne.Window) *Wizard {
	w := &Wizard{
		window:       window,
		currentStep:  StepInput,
		stepContents: make(map[WizardStep]fyne.CanvasObject),
	}

	w.createNavButtons()
	w.createStepIndicator()

	return w
}

// createNavButtons creates the navigation buttons
func (w *Wizard) createNavButtons() {
	w.backButton = widget.NewButton("Back", func() {
		w.Previous()
	})

	w.nextButton = widget.NewButton("Next", func() {
		w.Next()
	})
	w.nextButton.Importance = widget.HighImportance

	// Initially hide back button on first step
	w.backButton.Disable()
}

// createStepIndicator creates the step indicator UI
func (w *Wizard) createStepIndicator() {
	w.stepIndicators = make([]*canvas.Circle, len(stepInfos))
	w.stepLabels = make([]*canvas.Text, len(stepInfos))

	var items []fyne.CanvasObject

	for i, info := range stepInfos {
		// Create circle for step indicator
		circle := canvas.NewCircle(ColorStepInactive)
		circle.StrokeColor = ColorBorder
		circle.StrokeWidth = 2
		w.stepIndicators[i] = circle

		// Create label for step
		label := canvas.NewText(info.Title, ColorTextSecondary)
		label.TextSize = 12
		label.Alignment = fyne.TextAlignCenter
		w.stepLabels[i] = label

		// Container for circle and label (both centered)
		circleContainer := container.New(&stepCircleLayout{}, circle)
		stepItem := container.NewVBox(
			container.NewCenter(circleContainer),
			container.NewCenter(label),
		)

		items = append(items, stepItem)

		// Add connecting line between steps (except after last)
		if i < len(stepInfos)-1 {
			line := canvas.NewRectangle(ColorBorder)
			lineContainer := container.New(&stepLineLayout{}, line)
			items = append(items, lineContainer)
		}
	}

	w.stepIndicator = container.NewHBox(items...)
	w.updateStepIndicator()
}

// stepCircleLayout is a custom layout for step indicator circles
type stepCircleLayout struct{}

func (l *stepCircleLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(24, 24)
}

func (l *stepCircleLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(24, 24))
		o.Move(fyne.NewPos(0, 0))
	}
}

// stepLineLayout is a custom layout for connecting lines
type stepLineLayout struct{}

func (l *stepLineLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(40, 24)
}

func (l *stepLineLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(40, 2))
		o.Move(fyne.NewPos(0, 11))
	}
}

// updateStepIndicator updates the visual state of step indicators
func (w *Wizard) updateStepIndicator() {
	for i := range stepInfos {
		step := WizardStep(i)
		if step < w.currentStep {
			// Completed step
			w.stepIndicators[i].FillColor = ColorStepComplete
			w.stepIndicators[i].StrokeColor = ColorStepComplete
			w.stepLabels[i].Color = ColorTextPrimary
		} else if step == w.currentStep {
			// Current step
			w.stepIndicators[i].FillColor = ColorPrimaryAccent
			w.stepIndicators[i].StrokeColor = ColorPrimaryAccent
			w.stepLabels[i].Color = ColorTextPrimary
		} else {
			// Future step
			w.stepIndicators[i].FillColor = ColorStepInactive
			w.stepIndicators[i].StrokeColor = ColorBorder
			w.stepLabels[i].Color = ColorTextSecondary
		}
		w.stepIndicators[i].Refresh()
		w.stepLabels[i].Refresh()
	}
}

// SetStepContent sets the content for a specific step
func (w *Wizard) SetStepContent(step WizardStep, content fyne.CanvasObject) {
	w.stepContents[step] = content
}

// SetOnStepChange sets the callback for when the step changes
func (w *Wizard) SetOnStepChange(callback func(WizardStep)) {
	w.onStepChange = callback
}

// SetCanProceed sets the validation callback for step transitions
func (w *Wizard) SetCanProceed(callback func(WizardStep) bool) {
	w.canProceed = callback
}

// SetStatusIndicator sets an optional status indicator to display in the header
func (w *Wizard) SetStatusIndicator(indicator fyne.CanvasObject) {
	w.statusIndicator = indicator
}

// SetDcmtkInstalled updates the dcmtk installation status
func (w *Wizard) SetDcmtkInstalled(installed bool) {
	w.dcmtkInstalled = installed
	w.updateNavButtons()
}

// SetOnDcmtkWarning sets the callback for when user tries to proceed without dcmtk
// The callback receives a proceed function that should be called to continue navigation
func (w *Wizard) SetOnDcmtkWarning(callback func(proceed func())) {
	w.onDcmtkWarning = callback
}

// IsDcmtkInstalled returns whether dcmtk is installed
func (w *Wizard) IsDcmtkInstalled() bool {
	return w.dcmtkInstalled
}

// Next moves to the next step
func (w *Wizard) Next() {
	if w.canProceed != nil && !w.canProceed(w.currentStep) {
		return
	}

	// Show dcmtk warning if on step 1 and dcmtk is not installed
	if w.currentStep == StepInput && !w.dcmtkInstalled && w.onDcmtkWarning != nil {
		w.onDcmtkWarning(func() {
			// User chose to continue anyway
			if w.currentStep < StepProcess {
				w.GoToStep(w.currentStep + 1)
			}
		})
		return
	}

	if w.currentStep < StepProcess {
		w.GoToStep(w.currentStep + 1)
	}
}

// Previous moves to the previous step
func (w *Wizard) Previous() {
	if w.currentStep > StepInput {
		w.GoToStep(w.currentStep - 1)
	}
}

// GoToStep navigates to a specific step
func (w *Wizard) GoToStep(step WizardStep) {
	if step < StepInput || step > StepProcess {
		return
	}

	w.currentStep = step
	w.updateStepIndicator()
	w.updateNavButtons()
	w.updateContent()

	if w.onStepChange != nil {
		w.onStepChange(step)
	}
}

// GetCurrentStep returns the current wizard step
func (w *Wizard) GetCurrentStep() WizardStep {
	return w.currentStep
}

// updateNavButtons updates the navigation button states
func (w *Wizard) updateNavButtons() {
	// Back button
	if w.currentStep == StepInput {
		w.backButton.Disable()
	} else {
		w.backButton.Enable()
	}

	// Next button
	switch w.currentStep {
	case StepProcess:
		w.nextButton.SetText("Done")
		w.nextButton.Disable() // Will be enabled when processing completes
	case StepPreview:
		w.nextButton.SetText("Process")
	default:
		w.nextButton.SetText("Next")
		w.nextButton.Enable()
	}
}

// updateContent shows the content for the current step
func (w *Wizard) updateContent() {
	if w.contentContainer == nil {
		return
	}

	// Remove current content
	w.contentContainer.Objects = nil

	// Add new content
	if content, ok := w.stepContents[w.currentStep]; ok {
		w.contentContainer.Objects = []fyne.CanvasObject{content}
	}

	w.contentContainer.Refresh()
}

// SetNextEnabled enables or disables the next button
func (w *Wizard) SetNextEnabled(enabled bool) {
	if enabled {
		w.nextButton.Enable()
	} else {
		w.nextButton.Disable()
	}
}

// SetNextText sets the text of the next button
func (w *Wizard) SetNextText(text string) {
	w.nextButton.SetText(text)
}

// SetBackEnabled enables or disables the back button
func (w *Wizard) SetBackEnabled(enabled bool) {
	if enabled && w.currentStep > StepInput {
		w.backButton.Enable()
	} else {
		w.backButton.Disable()
	}
}

// Build creates the complete wizard UI
func (w *Wizard) Build() fyne.CanvasObject {
	// Content area with padding
	w.contentContainer = container.NewStack()

	// Add initial content
	if content, ok := w.stepContents[w.currentStep]; ok {
		w.contentContainer.Objects = []fyne.CanvasObject{content}
	}

	// Card background for content
	contentBg := canvas.NewRectangle(ColorCardBackground)
	contentBg.CornerRadius = 8

	contentCard := container.NewStack(
		contentBg,
		container.NewPadded(w.contentContainer),
	)

	// Navigation row
	navRow := container.NewBorder(
		nil, nil,
		w.backButton,
		w.nextButton,
		layout.NewSpacer(),
	)

	// Step indicator centered
	stepIndicatorCentered := container.NewCenter(w.stepIndicator)

	// Separator line
	separator := canvas.NewRectangle(ColorBorder)
	separator.SetMinSize(fyne.NewSize(0, 1))

	// Navigation row with status indicator in the center
	var bottomRow fyne.CanvasObject
	if w.statusIndicator != nil {
		bottomRow = container.NewBorder(
			nil, nil,
			w.backButton,
			w.nextButton,
			container.NewCenter(w.statusIndicator),
		)
	} else {
		bottomRow = navRow
	}

	// Main layout
	mainContent := container.NewBorder(
		container.NewVBox(
			container.NewPadded(stepIndicatorCentered),
			separator,
		),
		container.NewPadded(bottomRow),
		nil, nil,
		container.NewPadded(contentCard),
	)

	return mainContent
}

// createCard creates a styled card container
func createCard(title string, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(ColorCardBackground)
	bg.CornerRadius = 8

	var header fyne.CanvasObject
	if title != "" {
		titleLabel := canvas.NewText(title, ColorTextPrimary)
		titleLabel.TextSize = 16
		titleLabel.TextStyle = fyne.TextStyle{Bold: true}
		header = container.NewVBox(
			titleLabel,
			canvas.NewRectangle(color.Transparent),
		)
	}

	return container.NewStack(
		bg,
		container.NewPadded(
			container.NewBorder(header, nil, nil, nil, content),
		),
	)
}
