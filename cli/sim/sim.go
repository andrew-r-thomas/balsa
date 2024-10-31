package sim

import (
	"bufio"
	"log"
	"net/http"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Sim struct {
	Id   int
	Name string
}

type Model struct {
	sim    Sim
	logMsg string
}

func InitialModel(sim Sim) Model {
	// TODO: make this cross platform
	startNodeCmd := exec.Command("./balsa", "localhost:50051", "localhost:3000")
	err := startNodeCmd.Start()
	if err != nil {
		log.Fatalf("error starting node: %v", err)
	}
	time.Sleep(time.Second)
	resp, err := http.Get("http://localhost:3000/logs")
	if err != nil {
		log.Fatalf("error getting log: %v", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Scan()
	logMsg := scanner.Text()

	return Model{sim: sim, logMsg: logMsg}
}
func (m Model) Init() tea.Cmd {
	return nil
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}
func (m Model) View() string {
	var pstr string
	if m.sim.Id != 0 {
		pstr = "persistent "
	} else {
		pstr = "temporary "
	}
	return "sim view\n\n" + pstr + "sim: " + m.sim.Name + "\n\nlog msg: " + m.logMsg
}
