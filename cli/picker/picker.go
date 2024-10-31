package picker

import (
	"balsa/cli/newsim"
	"balsa/cli/sim"
	"database/sql"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const defaultWidth = 20
const listHeight = 14

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
)

type item struct {
	sim sim.Sim
}

func (i item) FilterValue() string { return i.sim.Name }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("- %s", i.sim.Name)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

type Model struct {
	list     list.Model
	dbHandle *sql.DB
}

func InitialModel(dbHandle *sql.DB) Model {
	sims, simsErr := dbHandle.Query("select * from sims")
	if simsErr != nil {
		log.Fatalf("unable to make open sims dir: %v", simsErr)
	}
	defer sims.Close()

	var items []list.Item
	for sims.Next() {
		var name string
		var id int
		simsErr = sims.Scan(&id, &name)
		if simsErr != nil {
			log.Fatal(simsErr)
		}
		items = append(items, item{sim: sim.Sim{Id: id, Name: name}})
	}
	simsErr = sims.Err()
	if simsErr != nil {
		log.Fatal(simsErr)
	}

	list := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	list.Title = "select a sim to run or create a new one"
	list.SetShowStatusBar(false)
	list.SetFilteringEnabled(false)
	list.Styles.Title = titleStyle
	list.Styles.PaginationStyle = paginationStyle
	list.Styles.HelpStyle = helpStyle
	return Model{
		list:     list,
		dbHandle: dbHandle,
	}
}
func (m Model) Init() tea.Cmd {
	return nil
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "n":
			return newsim.InitialModel(m.dbHandle), nil

		case "enter":
			i := m.list.SelectedItem().(item)
			return sim.InitialModel(i.sim), nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}
func (m Model) View() string {
	return "\n" + m.list.View()
}
