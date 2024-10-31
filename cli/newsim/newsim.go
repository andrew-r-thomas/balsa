package newsim

import (
	"balsa/cli/sim"
	"database/sql"
	"log"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	name     textinput.Model
	nameSet  bool
	dbHandle *sql.DB
}

func InitialModel(dbHandle *sql.DB) Model {
	name := textinput.New()
	name.Prompt = "new sim name: "
	name.Width = 20
	name.Focus()

	return Model{name: name, nameSet: false, dbHandle: dbHandle}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if !m.nameSet {
		// TODO: probably want to get rid of "q" to quit here
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch keypress := msg.String(); keypress {
			case "enter", "tab":
				m.nameSet = true
				m.name.Cursor.SetMode(cursor.CursorHide)
				return m, cmd
			}
		}
		m.name, cmd = m.name.Update(msg)
		return m, cmd
	} else {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch keypress := msg.String(); keypress {
			case "y", "Y":
				stmt, err := m.dbHandle.Prepare("insert into sims(name) values(?) returning *;")
				if err != nil {
					log.Fatalf("new sim prep error: %v", err)
				}
				defer stmt.Close()
				var name string
				var id int
				err = stmt.QueryRow(m.name.Value()).Scan(&id, &name)
				if err != nil {
					log.Fatalf("new sim query error: %v", err)
				}

				return sim.InitialModel(sim.Sim{Id: id, Name: name}), nil
			case "n", "N":
				return sim.InitialModel(sim.Sim{Id: 0, Name: m.name.Value()}), nil
			}
		}
	}
	return m, cmd
}
func (m Model) View() string {
	if m.nameSet {
		return m.name.View() + "\nshould this sim be persistent? (y/n)"
	} else {
		return m.name.View()
	}
}
