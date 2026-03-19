package tui

import "github.com/charmbracelet/lipgloss"

var (
	Title     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	Subtitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	Help      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	Selected  = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	Error     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	StatusBar = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
	Concealed = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)
