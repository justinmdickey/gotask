package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}

	titleStyle = lipgloss.NewStyle().
			MarginLeft(1).
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	columnHeaderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderBottom(true).
				BorderForeground(highlight).
				Foreground(highlight).
				Bold(true).
				Padding(0, 1)

	columnStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(1, 2)

	itemStyle = lipgloss.NewStyle().PaddingLeft(4)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(special).
				SetString("❯ ")

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 0).
			Width(30).
			Height(3)
)

// Task represents a single task in our kanban board
type Task struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Column represents a column in our kanban board
type Column struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Tasks []Task `json:"tasks"`
}

// KanbanBoard represents our entire kanban board
type KanbanBoard struct {
	Columns []Column `json:"columns"`
}

// Model holds the application state
type model struct {
	board         KanbanBoard
	cursorColumn  int
	cursorTask    int
	textInput     textinput.Model
	inputMode     bool
	width         int
	height        int
	err           error
	savePath      string
	lastID        int
	showTaskInput bool
	showHelp      bool
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Add a new task..."
	ti.Focus()

	homedir, err := os.UserHomeDir()
	if err != nil {
		homedir = "."
	}
	savePath := filepath.Join(homedir, ".kanban.json")

	m := model{
		board: KanbanBoard{
			Columns: []Column{
				{ID: 1, Title: "To Do", Tasks: []Task{}},
				{ID: 2, Title: "In Progress", Tasks: []Task{}},
				{ID: 3, Title: "Done", Tasks: []Task{}},
			},
		},
		textInput:    ti,
		inputMode:    false,
		savePath:     savePath,
		lastID:       0,
		showTaskInput: false,
		showHelp:     true,
	}

	// Try to load existing data
	if err := m.loadBoard(); err != nil {
		m.err = err
	}

	return m
}

func (m *model) loadBoard() error {
	data, err := ioutil.ReadFile(m.savePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, that's fine
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, &m.board); err != nil {
		return err
	}

	// Find the highest task ID
	for _, col := range m.board.Columns {
		for _, task := range col.Tasks {
			if task.ID > m.lastID {
				m.lastID = task.ID
			}
		}
	}

	return nil
}

func (m *model) saveBoard() error {
	data, err := json.MarshalIndent(m.board, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(m.savePath, data, 0644)
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if err := m.saveBoard(); err != nil {
				m.err = err
				return m, nil
			}
			return m, tea.Quit

		case "?":
			m.showHelp = !m.showHelp
			return m, nil

		case "enter":
			if m.inputMode {
				if m.textInput.Value() != "" {
					m.lastID++
					newTask := Task{
						ID:        m.lastID,
						Title:     m.textInput.Value(),
						CreatedAt: time.Now(),
					}
					col := &m.board.Columns[m.cursorColumn]
					col.Tasks = append(col.Tasks, newTask)
					m.textInput.Reset()
					m.inputMode = false
					if err := m.saveBoard(); err != nil {
						m.err = err
					}
				} else {
					m.inputMode = false
				}
			}

		case "a":
			if !m.inputMode {
				m.inputMode = true
				return m, textinput.Blink
			}

		case "d":
			if !m.inputMode && len(m.board.Columns) > 0 {
				col := &m.board.Columns[m.cursorColumn]
				if len(col.Tasks) > 0 {
					// Delete task
					col.Tasks = append(col.Tasks[:m.cursorTask], col.Tasks[m.cursorTask+1:]...)
					if m.cursorTask >= len(col.Tasks) && m.cursorTask > 0 {
						m.cursorTask--
					}
					if err := m.saveBoard(); err != nil {
						m.err = err
					}
				}
			}

		case "esc":
			if m.inputMode {
				m.inputMode = false
				m.textInput.Reset()
			}

		case "up", "k":
			if !m.inputMode {
				col := &m.board.Columns[m.cursorColumn]
				if len(col.Tasks) > 0 {
					m.cursorTask = max(0, m.cursorTask-1)
				}
			}

		case "down", "j":
			if !m.inputMode {
				col := &m.board.Columns[m.cursorColumn]
				if len(col.Tasks) > 0 {
					m.cursorTask = min(len(col.Tasks)-1, m.cursorTask+1)
				}
			}

		case "left", "h":
			if !m.inputMode {
				if m.cursorColumn > 0 {
					m.cursorColumn--
					m.cursorTask = 0
				}
			}

		case "right", "l":
			if !m.inputMode {
				if m.cursorColumn < len(m.board.Columns)-1 {
					m.cursorColumn++
					m.cursorTask = 0
				}
			}

		case "[", "{":
			// Move task left if possible
			if !m.inputMode && m.cursorColumn > 0 {
				srcCol := &m.board.Columns[m.cursorColumn]
				if len(srcCol.Tasks) > 0 {
					destCol := &m.board.Columns[m.cursorColumn-1]
					task := srcCol.Tasks[m.cursorTask]
					
					// Remove from source
					srcCol.Tasks = append(srcCol.Tasks[:m.cursorTask], srcCol.Tasks[m.cursorTask+1:]...)
					if m.cursorTask >= len(srcCol.Tasks) && m.cursorTask > 0 {
						m.cursorTask--
					}
					
					// Add to destination
					destCol.Tasks = append(destCol.Tasks, task)
					
					// Move cursor to the destination column
					m.cursorColumn--
					m.cursorTask = len(destCol.Tasks) - 1
					
					if err := m.saveBoard(); err != nil {
						m.err = err
					}
				}
			}

		case "]", "}":
			// Move task right if possible
			if !m.inputMode && m.cursorColumn < len(m.board.Columns)-1 {
				srcCol := &m.board.Columns[m.cursorColumn]
				if len(srcCol.Tasks) > 0 {
					destCol := &m.board.Columns[m.cursorColumn+1]
					task := srcCol.Tasks[m.cursorTask]
					
					// Remove from source
					srcCol.Tasks = append(srcCol.Tasks[:m.cursorTask], srcCol.Tasks[m.cursorTask+1:]...)
					if m.cursorTask >= len(srcCol.Tasks) && m.cursorTask > 0 {
						m.cursorTask--
					}
					
					// Add to destination
					destCol.Tasks = append(destCol.Tasks, task)
					
					// Move cursor to the destination column
					m.cursorColumn++
					m.cursorTask = len(destCol.Tasks) - 1
					
					if err := m.saveBoard(); err != nil {
						m.err = err
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	if m.inputMode {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var s strings.Builder

	// Title
	title := titleStyle.Render(" KANBAN BOARD ")
	s.WriteString(title + "\n\n")

	// Calculate column width based on available space and number of columns
	columnWidth := (m.width / len(m.board.Columns)) - 5

	// Prepare columns for rendering
	renderedColumns := make([]string, len(m.board.Columns))
	for i, col := range m.board.Columns {
		var column strings.Builder

		// Column header
		column.WriteString(columnHeaderStyle.Render(col.Title))
		column.WriteString("\n\n")

		// Tasks
		if len(col.Tasks) == 0 {
			column.WriteString(itemStyle.Render("No tasks"))
		} else {
			for j, task := range col.Tasks {
				taskLine := task.Title
				if m.cursorColumn == i && m.cursorTask == j {
					taskLine = selectedItemStyle.String() + taskLine
				} else {
					taskLine = "  " + taskLine
				}
				column.WriteString(taskLine + "\n")
			}
		}

		renderedColumns[i] = columnStyle.Width(columnWidth).Render(column.String())
	}

	// Join columns side by side
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, renderedColumns...))

	// Input field
	if m.inputMode {
		dialog := dialogBoxStyle.Render("New task in " + m.board.Columns[m.cursorColumn].Title + ":\n" + m.textInput.View())
		s.WriteString("\n\n" + dialog)
	}

	// Error message
	if m.err != nil {
		s.WriteString("\n\nError: " + m.err.Error())
	}

	// Help
	if m.showHelp {
		help := "\n\n" + helpStyle.Render(
			"a: add task • d: delete task • [/]: move task left/right • arrow keys: navigate • ?: toggle help • q: quit",
		)
		s.WriteString(help)
	}

	return s.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}

