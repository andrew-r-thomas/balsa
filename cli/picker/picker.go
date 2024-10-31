package picker

import (
	"fmt"
	"io"
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

// TODO: figure out wtf all this bullshit is doing
type item string                                               //
func (i item) FilterValue() string                             { return "" } //
type itemDelegate struct{}                                     //
func (d itemDelegate) Height() int                             { return 1 }   //
func (d itemDelegate) Spacing() int                            { return 0 }   //
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil } //
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) { //
	i, ok := listItem.(item) //
	if !ok {                 //
		return //
	}
	//
	str := fmt.Sprintf("%d. %s", index+1, i) //
	//
	fn := itemStyle.Render  //
	if index == m.Index() { //
		fn = func(s ...string) string { //
			return selectedItemStyle.Render("> " + strings.Join(s, " ")) //
		} //
	} //
	//
	fmt.Fprint(w, fn(str)) //
} //
//

type Model struct {
	list   list.Model
	choice string
}

func InitialModel(sims []string) Model {
	var items []list.Item
	for _, sim := range sims {
		items = append(items, item(sim))
	}
	list := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	list.Title = "select a sim to run or create a new one"
	list.SetShowStatusBar(false)
	list.SetFilteringEnabled(false)
	list.Styles.Title = titleStyle
	list.Styles.PaginationStyle = paginationStyle
	list.Styles.HelpStyle = helpStyle
	return Model{
		list: list,
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
		// TODO: switch to new sim view
		case "n":
			return m, nil

		// TODO: this is where we will select a sim
		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = string(i)
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}
func (m Model) View() string {
	return "\n" + m.list.View()
}
