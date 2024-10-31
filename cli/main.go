/*

	 TODO:
	- add help to picker page and new sim page
	- filtering for picker page
	- delete sims
	- make it pretty
	- get some kind of sim visual working (break this into smaller pieces)
	- figure out when we should do stuff in "InitialModel" and when we
	  should do stuff in "Init"
	- change io stuff to be driven via commands

	 NOTE:

*/

package main

import (
	"balsa/cli/picker"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"database/sql"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "github.com/mattn/go-sqlite3"
)

const createTables = `
create table sims (
	id integer primary key autoincrement,
	name text not null unique
);
`
const quitText = "\"A genuine leader is not a searcher for consensus but a molder of consensus.\"\n\n- Martin Luther King Jr."

// TODO: for making stuff pretty
var (
	quitTextStyle = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type model struct {
	quitting bool

	currentView tea.Model
	dbHandle    *sql.DB
}

func initialModel() model {
	var dbHandle *sql.DB
	simsPath := filepath.Join(".", "sims")
	_, simsDirErr := os.Stat(simsPath)
	if simsDirErr != nil {
		if os.IsNotExist(simsDirErr) {
			simsDirErr = os.Mkdir(simsPath, 0755)
			if simsDirErr != nil {
				log.Fatal(simsDirErr)
			}
			dbPath := filepath.Join(simsPath, ".db")
			db, dbErr := sql.Open("sqlite3", dbPath)
			if dbErr != nil {
				log.Fatal(dbErr)
			}
			_, err := db.Exec(createTables)
			if err != nil {
				log.Fatal(err)
			}
			dbHandle = db
		} else {
			log.Fatalf("sims directory error: %v", simsDirErr)
		}
	} else {
		dbPath := filepath.Join(simsPath, ".db")
		_, dbPathErr := os.Stat(dbPath)
		if dbPathErr != nil {
			if os.IsNotExist(dbPathErr) {
				dbPath := filepath.Join(simsPath, ".db")
				db, dbErr := sql.Open("sqlite3", dbPath)
				if dbErr != nil {
					log.Fatal(dbErr)
				}
				_, err := db.Exec(createTables)
				if err != nil {
					log.Fatal(err)
				}
				dbHandle = db
			} else {
				log.Fatalf("sims db error: %v", dbPathErr)
			}
		} else {
			db, dbErr := sql.Open("sqlite3", dbPath)
			if dbErr != nil {
				log.Fatalf("db open error: %v", dbErr)
			} else {
				dbHandle = db
			}
		}
	}

	return model{
		quitting:    false,
		currentView: picker.InitialModel(dbHandle),
		dbHandle:    dbHandle,
	}
}

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

	var cmd tea.Cmd
	m.currentView, cmd = m.currentView.Update(msg)
	return m, cmd
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
