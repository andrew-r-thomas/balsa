/*

	 TODO:
	- add help to picker page and new sim page
	- new sim page needs text input for name, and a switch for
	  whether or not its a temporary sim
	- filtering for picker page
	- delete sims
	- make it pretty
	- get some kind of sim visual working (break this into smaller pieces)

*/

/*

	 NOTE:
	we might want want to use a sqlite db to store info instead of the
	file system, and just have different dbs for each sim/node

*/

package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const quitText = "\"A genuine leader is not a searcher for consensus but a molder of consensus.\"\n\n- Martin Luther King Jr."

type Views int

const (
	PickerView Views = iota
	NewSimView
	SimView
)

// TODO: for making stuff pretty
var (
	quitTextStyle = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type model struct {
	quitting bool

	currentView tea.Model
}

func initialModel() model {
	_, noSims := os.Stat("./sims")
	if noSims != nil {
		err := os.Mkdir("./sims", 0755)
		if err != nil {
			log.Fatalf("unable to make sims dir: %v", err)
		}
	}

	// TODO: change this to db query
	sims, simsErr := os.ReadDir("./sims")
	if simsErr != nil {
		log.Fatalf("unable to make open sims dir: %v", simsErr)
	}

	return model{
		quitting: false,
	}
}

// TODO: maybe put loading stuff in here
func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m.currentView.Update(msg)
}

func (m model) View() string {
	if m.quitting {
		return quitTextStyle.Render(quitText)
	}

	return m.currentView.View()
}

func main() {
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
