package main

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	name textinput.Model
}

func InitialModel() Model {
	var name textinput.Model
	name.Prompt = "new sim name: "
	name.Focus()

	return Model{name: name}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// TODO:
	return m, nil
}
func (m Model) View() string {
	return "\n" + m.name.View()
}
