package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	// Terminal colors
	subtle      = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight   = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#7D56F4"} // Purple
	special     = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"} // Green
	todoColor   = lipgloss.AdaptiveColor{Light: "#E06C75", Dark: "#E06C75"} // Red
	inProgColor = lipgloss.AdaptiveColor{Light: "#E5C07B", Dark: "#E5C07B"} // Yellow
	doneColor   = lipgloss.AdaptiveColor{Light: "#98C379", Dark: "#98C379"} // Green

	titleStyle = lipgloss.NewStyle().
			MarginLeft(1).
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 2)

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

	todoColumnStyle = columnStyle.Copy().BorderForeground(todoColor)
	inProgColumnStyle = columnStyle.Copy().BorderForeground(inProgColor)
	doneColumnStyle = columnStyle.Copy().BorderForeground(doneColor)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4).
			PaddingBottom(1)

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
			
	confirmDialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#E06C75")).
			Padding(1, 0).
			Width(40).
			Height(5)
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

// InputMode represents different input modes (like vim)
type InputMode int

const (
	NormalMode InputMode = iota
	InsertMode
)

// DialogType represents different types of dialogs
type DialogType int

const (
	NoDialog DialogType = iota
	DeleteDialog
	EditDialog
)

// Model holds the application state
type model struct {
	board         KanbanBoard
	cursorColumn  int
	cursorTask    int
	textInput     textinput.Model
	inputMode     bool
	inputState    InputMode
	width         int
	height        int
	err           error
	savePath      string
	lastID        int
	showTaskInput bool
	showHelp      bool
	dialogType    DialogType
	editingTask   *Task
	viewports     []viewport.Model  // viewports for scrollable columns
	headerHeight  int               // height of the header section
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

	// Create initial viewports for columns
	viewports := make([]viewport.Model, 3)
	for i := range viewports {
		vp := viewport.New(0, 0)
		vp.MouseWheelEnabled = true
		viewports[i] = vp
	}

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
		inputState:   NormalMode,
		savePath:     savePath,
		lastID:       0,
		showTaskInput: false,
		showHelp:     true,
		dialogType:   NoDialog,
		editingTask:  nil,
		viewports:    viewports,
		headerHeight: 5, // Fixed height for title (1) + padding (2) + column headers (1) + padding (1)
	}

	// Try to load existing data
	if err := m.loadBoard(); err != nil {
		m.err = err
	}

	return m
}

func (m *model) loadBoard() error {
	data, err := os.ReadFile(m.savePath)
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

	return os.WriteFile(m.savePath, data, 0644)
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Update all viewports
	for i := range m.viewports {
		var cmd tea.Cmd
		m.viewports[i], cmd = m.viewports[i].Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle delete confirmation dialog
		if m.dialogType == DeleteDialog {
			switch msg.String() {
			case "y", "Y":
				// Confirm deletion
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
				m.dialogType = NoDialog
				return m, nil
			case "n", "N", "esc", "q", "ctrl+c":
				// Cancel deletion
				m.dialogType = NoDialog
				return m, nil
			default:
				return m, nil
			}
		}
		
		// Handle input based on current mode
		if m.inputMode {
			switch m.inputState {
			case NormalMode:
				// In normal mode, handle vim-like commands
				switch msg.String() {
				case "i":
					// Switch to insert mode
					m.inputState = InsertMode
					return m, nil
				
				case "esc", "ctrl+c":
					// Exit input mode
					m.inputMode = false
					m.textInput.Reset()
					m.inputState = NormalMode
					m.editingTask = nil
					m.dialogType = NoDialog
					return m, nil
					
				case "enter":
					if m.dialogType == EditDialog && m.editingTask != nil {
						// Update the task
						m.editingTask.Title = m.textInput.Value()
						m.inputMode = false
						m.inputState = NormalMode
						m.editingTask = nil
						m.dialogType = NoDialog
						if err := m.saveBoard(); err != nil {
							m.err = err
						}
						return m, nil
					}
					
					// Submit the task if it's not empty
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
						m.inputState = NormalMode
						if err := m.saveBoard(); err != nil {
							m.err = err
						}
					} else {
						m.inputMode = false
						m.inputState = NormalMode
					}
					return m, nil
				
				// Allow navigation while in normal mode
				case "q":
					if err := m.saveBoard(); err != nil {
						m.err = err
						return m, nil
					}
					return m, tea.Quit
					
				case "?":
					m.showHelp = !m.showHelp
					return m, nil
				}
				
			case InsertMode:
				// In insert mode, handle text input normally
				switch msg.String() {
				case "esc":
					// Switch back to normal mode
					m.inputState = NormalMode
					return m, nil
					
				case "enter":
					if m.dialogType == EditDialog && m.editingTask != nil {
						// Update the task
						m.editingTask.Title = m.textInput.Value()
						m.inputMode = false
						m.inputState = NormalMode
						m.editingTask = nil
						m.dialogType = NoDialog
						if err := m.saveBoard(); err != nil {
							m.err = err
						}
						return m, nil
					}
					
					// Submit the task if it's not empty
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
						m.inputState = NormalMode
						if err := m.saveBoard(); err != nil {
							m.err = err
						}
					} else {
						m.inputMode = false
						m.inputState = NormalMode
					}
					return m, nil
				
				default:
					// Update text input normally in insert mode
					var cmd tea.Cmd
					m.textInput, cmd = m.textInput.Update(msg)
					return m, cmd
				}
			}
			
			// Default case for handling input in any mode
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		} else {
			// When not in input mode, handle normal application commands
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
				
			case "a":
				// Enter input mode in insert mode
				m.inputMode = true
				m.inputState = InsertMode
				m.textInput.Reset()
				return m, textinput.Blink
				
			case "n":
				// Enter input mode in normal mode 
				m.inputMode = true
				m.inputState = NormalMode
				m.textInput.Reset()
				return m, textinput.Blink

			case "e":
				if len(m.board.Columns) > 0 {
					col := &m.board.Columns[m.cursorColumn]
					if len(col.Tasks) > 0 {
						// Enter edit mode
						m.dialogType = EditDialog
						m.editingTask = &col.Tasks[m.cursorTask]
						m.textInput.SetValue(m.editingTask.Title)
						m.inputMode = true
						m.inputState = InsertMode
						return m, textinput.Blink
					}
				}
				
			case "d":
				if len(m.board.Columns) > 0 {
					col := &m.board.Columns[m.cursorColumn]
					if len(col.Tasks) > 0 {
						// Show delete confirmation dialog
						m.dialogType = DeleteDialog
						return m, nil
					}
				}

			case "up", "k":
				col := &m.board.Columns[m.cursorColumn]
				if len(col.Tasks) > 0 {
					m.cursorTask = max(0, m.cursorTask-1)
					m.updateViewportContent(m.cursorColumn)
				}

			case "down", "j":
				col := &m.board.Columns[m.cursorColumn]
				if len(col.Tasks) > 0 {
					m.cursorTask = min(len(col.Tasks)-1, m.cursorTask+1)
					m.updateViewportContent(m.cursorColumn)
				}

			case "left", "h":
				if m.cursorColumn > 0 {
					m.cursorColumn--
					m.cursorTask = 0
					m.updateViewportContent(m.cursorColumn)
				}

			case "right", "l":
				if m.cursorColumn < len(m.board.Columns)-1 {
					m.cursorColumn++
					m.cursorTask = 0
					m.updateViewportContent(m.cursorColumn)
				}

			case "[", "{":
				// Move task left if possible
				if m.cursorColumn > 0 {
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
						
						// Update viewport content for both columns
						m.updateViewportContent(m.cursorColumn)
						m.updateViewportContent(m.cursorColumn+1)
						
						if err := m.saveBoard(); err != nil {
							m.err = err
						}
					}
				}

			case "]", "}":
				// Move task right if possible
				if m.cursorColumn < len(m.board.Columns)-1 {
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
						
						// Update viewport content for both columns
						m.updateViewportContent(m.cursorColumn)
						m.updateViewportContent(m.cursorColumn-1)
						
						if err := m.saveBoard(); err != nil {
							m.err = err
						}
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Update the fixed header height
		m.headerHeight = 5 // Title (1) + padding (2) + column headers (1) + padding (1)
		
		// Calculate column width based on available space and number of columns
		columnWidth := (m.width / len(m.board.Columns)) - 5
		
		// Update the viewports with new dimensions
		// The height is calculated by subtracting header, help text, and any other UI elements
		viewportHeight := m.height - m.headerHeight
		if m.showHelp {
			viewportHeight -= 3 // Subtract height of help text
		}
		
		// Make sure viewport height has a reasonable minimum
		viewportHeight = max(10, viewportHeight)
		
		// Resize all viewports
		for i := range m.viewports {
			// Set viewport size
			m.viewports[i].Width = columnWidth
			m.viewports[i].Height = viewportHeight
			
			// Update content for each viewport
			m.updateViewportContent(i)
		}
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var s strings.Builder

	// Title - centered based on terminal width
	title := titleStyle.Render(" KANBAN BOARD ")
	paddingLeft := strings.Repeat(" ", (m.width-lipgloss.Width(title))/2)
	s.WriteString(paddingLeft + title + "\n\n")

	// Calculate column width based on available space and number of columns
	columnWidth := (m.width / len(m.board.Columns)) - 5

	// Render column headers separately for sticky header
	columnHeaders := make([]string, len(m.board.Columns))
	for i, col := range m.board.Columns {
		// Column header with color based on column type
		var headerStyle lipgloss.Style
		switch i {
		case 0: // To Do
			headerStyle = columnHeaderStyle.Copy().BorderForeground(todoColor).Foreground(todoColor)
		case 1: // In Progress
			headerStyle = columnHeaderStyle.Copy().BorderForeground(inProgColor).Foreground(inProgColor)
		case 2: // Done
			headerStyle = columnHeaderStyle.Copy().BorderForeground(doneColor).Foreground(doneColor)
		default:
			headerStyle = columnHeaderStyle
		}
		columnHeaders[i] = headerStyle.Width(columnWidth).Render(col.Title)
	}

	// Join headers side by side
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Bottom, columnHeaders...) + "\n\n")
	
	// Prepare columns for rendering (only task content, not headers)
	renderedColumns := make([]string, len(m.board.Columns))
	for i, _ := range m.board.Columns {
		// Apply the appropriate column style based on the column
		var colStyle lipgloss.Style
		switch i {
		case 0: // To Do
			colStyle = todoColumnStyle
		case 1: // In Progress
			colStyle = inProgColumnStyle
		case 2: // Done
			colStyle = doneColumnStyle
		default:
			colStyle = columnStyle
		}

		// Now use the viewport for task content only
		renderedColumns[i] = colStyle.Width(columnWidth).Render(m.viewports[i].View())
	}

	// Join columns side by side
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, renderedColumns...))

	// Show delete confirmation dialog if active
	if m.dialogType == DeleteDialog {
		col := m.board.Columns[m.cursorColumn]
		task := col.Tasks[m.cursorTask]
		dialogContent := fmt.Sprintf("Delete task?\n\n%s\n\n[y/n]", task.Title)
		dialog := confirmDialogStyle.Render(dialogContent)
		
		// Center the dialog box
		dialogWidth := lipgloss.Width(dialog)
		dialogPosX := (m.width - dialogWidth) / 2
		dialogPosY := m.height / 3
		
		// Add padding to position the dialog
		paddingTop := strings.Repeat("\n", dialogPosY)
		paddingLeft := strings.Repeat(" ", dialogPosX)
		
		s.WriteString("\n\n" + paddingTop + paddingLeft + dialog)
		return s.String()
	}

	// Input field for adding/editing tasks
	if m.inputMode {
		modeIndicator := ""
		dialogTitle := ""
		
		// Set appropriate title and indicator based on whether we're editing or adding
		if m.dialogType == EditDialog {
			dialogTitle = "Edit task:"
		} else {
			dialogTitle = "New task in " + m.board.Columns[m.cursorColumn].Title + ":"
		}
		
		if m.inputState == InsertMode {
			modeIndicator = lipgloss.NewStyle().Foreground(special).Render("[INSERT MODE]")
		} else {
			modeIndicator = lipgloss.NewStyle().Foreground(todoColor).Render("[NORMAL MODE]")
		}
		
		dialog := dialogBoxStyle.Render(dialogTitle + "\n" + 
			m.textInput.View() + "\n" + modeIndicator)
		s.WriteString("\n\n" + dialog)
	}

	// Error message
	if m.err != nil {
		s.WriteString("\n\nError: " + lipgloss.NewStyle().Foreground(lipgloss.Color("#E06C75")).Render(m.err.Error()))
	}

	// Help
	if m.showHelp {
		help := "\n\n" + helpStyle.Render(
			"a: add task • e: edit task • d: delete task • [/]: move task left/right • arrow keys: navigate • ?: toggle help • q: quit" +
			"\nWhen adding/editing: ESC: cancel • Enter: save task",
		)
		s.WriteString(help)
	}

	return s.String()
}

// Helper method to update the content of a viewport
func (m *model) updateViewportContent(columnIndex int) {
	columnWidth := (m.width / len(m.board.Columns)) - 15 // Adjusted for padding and borders
	
	var content strings.Builder
	
	// Only render tasks in the viewport
	col := m.board.Columns[columnIndex]
	if len(col.Tasks) == 0 {
		content.WriteString(itemStyle.Render("No tasks"))
	} else {
		for j, task := range col.Tasks {
			taskLine := task.Title
			if m.cursorColumn == columnIndex && m.cursorTask == j {
				taskLine = selectedItemStyle.String() + taskLine
			} else {
				taskLine = "  " + taskLine
			}
			
			// Add a border around each task for better separation with column-specific colors
			var taskBorderColor lipgloss.AdaptiveColor
			switch columnIndex {
			case 0: // To Do
				taskBorderColor = todoColor
			case 1: // In Progress
				taskBorderColor = inProgColor
			case 2: // Done
				taskBorderColor = doneColor
			default:
				taskBorderColor = subtle
			}
			
			taskBox := lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(taskBorderColor).
				Padding(0, 1).
				Width(columnWidth).
				Render(taskLine)
			
			content.WriteString(taskBox + "\n")
		}
	}
	
	// Set the viewport content
	m.viewports[columnIndex].SetContent(content.String())
	
	// Update scrolling position to show the selected task
	if m.cursorColumn == columnIndex && len(col.Tasks) > 0 {
		// Approximate height of a task box
		taskHeight := 3 // border top/bottom + content
		targetPos := m.cursorTask * taskHeight
		m.viewports[columnIndex].SetYOffset(targetPos)
	}
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