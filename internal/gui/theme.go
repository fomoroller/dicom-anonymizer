package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Modern dark theme colors
var (
	ColorBackground      = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x2E, A: 0xFF} // #1E1E2E
	ColorCardBackground  = color.NRGBA{R: 0x2A, G: 0x2A, B: 0x3E, A: 0xFF} // #2A2A3E
	ColorPrimaryAccent   = color.NRGBA{R: 0x89, G: 0xB4, B: 0xFA, A: 0xFF} // #89B4FA
	ColorSuccess         = color.NRGBA{R: 0xA6, G: 0xE3, B: 0xA1, A: 0xFF} // #A6E3A1
	ColorWarning         = color.NRGBA{R: 0xF9, G: 0xE2, B: 0xAF, A: 0xFF} // #F9E2AF
	ColorError           = color.NRGBA{R: 0xF3, G: 0x8B, B: 0xA8, A: 0xFF} // #F38BA8
	ColorTextPrimary     = color.NRGBA{R: 0xCD, G: 0xD6, B: 0xF4, A: 0xFF} // #CDD6F4
	ColorTextSecondary   = color.NRGBA{R: 0xA6, G: 0xAD, B: 0xC8, A: 0xFF} // #A6ADC8
	ColorDisabled        = color.NRGBA{R: 0x58, G: 0x5B, B: 0x70, A: 0xFF} // #585B70
	ColorInputBackground = color.NRGBA{R: 0x31, G: 0x32, B: 0x44, A: 0xFF} // #313244
	ColorBorder          = color.NRGBA{R: 0x45, G: 0x47, B: 0x5A, A: 0xFF} // #45475A
	ColorHover           = color.NRGBA{R: 0x6E, G: 0x9A, B: 0xE0, A: 0xFF} // #6E9AE0 - slightly darker blue for hover
	ColorStepInactive    = color.NRGBA{R: 0x45, G: 0x47, B: 0x5A, A: 0xFF} // #45475A
	ColorStepComplete    = color.NRGBA{R: 0xA6, G: 0xE3, B: 0xA1, A: 0xFF} // #A6E3A1
	ColorStatusGreen     = color.NRGBA{R: 0x40, G: 0xC0, B: 0x57, A: 0xFF} // #40C057 - bright green
	ColorStatusRed       = color.NRGBA{R: 0xFA, G: 0x52, B: 0x52, A: 0xFF} // #FA5252 - bright red
)

// ModernTheme implements the modern dark theme
type ModernTheme struct{}

var _ fyne.Theme = (*ModernTheme)(nil)

// Color returns the color for the given theme color name
func (m *ModernTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return ColorBackground
	case theme.ColorNameButton:
		return ColorPrimaryAccent
	case theme.ColorNameDisabledButton:
		return ColorDisabled
	case theme.ColorNameDisabled:
		return ColorDisabled
	case theme.ColorNameError:
		return ColorError
	case theme.ColorNameFocus:
		return ColorPrimaryAccent
	case theme.ColorNameForeground:
		return ColorTextPrimary
	case theme.ColorNameHeaderBackground:
		return ColorCardBackground
	case theme.ColorNameHover:
		return ColorHover
	case theme.ColorNameHyperlink:
		return ColorPrimaryAccent
	case theme.ColorNameInputBackground:
		return ColorInputBackground
	case theme.ColorNameInputBorder:
		return ColorBorder
	case theme.ColorNameMenuBackground:
		return ColorCardBackground
	case theme.ColorNameOverlayBackground:
		return ColorCardBackground
	case theme.ColorNamePlaceHolder:
		return ColorTextSecondary
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF} // Bright green #2ECC71
	case theme.ColorNamePrimary:
		return ColorPrimaryAccent
	case theme.ColorNameScrollBar:
		return ColorBorder
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x89, G: 0xB4, B: 0xFA, A: 0x66}
	case theme.ColorNameSeparator:
		return ColorBorder
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x66}
	case theme.ColorNameSuccess:
		return ColorSuccess
	case theme.ColorNameWarning:
		return ColorWarning
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

// Font returns the font for the given text style
func (m *ModernTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

// Icon returns the icon for the given icon name
func (m *ModernTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size returns the size for the given size name
func (m *ModernTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 12
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameInputBorder:
		return 2
	default:
		return theme.DefaultTheme().Size(name)
	}
}
