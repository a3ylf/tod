package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"todos/internal/todo"
)

type model struct {
	store       todo.Store
	path        string
	view        string
	focus       pane
	sidebar     int
	selected    int
	selectFrom  int
	taskIDs     []int
	width       int
	height      int
	search      string
	input       inputState
	inputUndo   []inputState
	editing     bool
	editTaskID  int
	editField   int
	message     string
	messageAt   time.Time
	confirmDel  bool
	exportID    int
	exportTitle string
	copyOnExit  bool
	undoStore   *todo.Store
}

type ExportedTask struct {
	ID     int
	Title  string
	Copied bool
	Count  int
}

type pane int

const (
	paneSidebar pane = iota
	paneTasks
)

type inputState struct {
	active      bool
	kind        string
	title       string
	value       string
	cursor      int
	selectStart int
	selecting   bool
}

type savedMsg struct {
	text string
	err  error
}

type copiedMsg struct {
	err error
}

var (
	titleStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	sectionStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	mutedStyle            = lipgloss.NewStyle().Faint(true)
	accentStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	projectInputStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	labelInputStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	dateInputStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	todayDateStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	futureDateStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	warnStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	selectedStyle         = lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.Color("15")).Bold(true)
	inactiveSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	selectedBorderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	multiBorderStyle      = selectedBorderStyle
	inactiveBorderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	borderStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	chipStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8")).Padding(0, 1)
	activeChipStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("9")).Bold(true).Padding(0, 1)
	cursorStyle           = lipgloss.NewStyle().Reverse(true)
)

var priorityStyles = map[int]lipgloss.Style{
	1: lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
	2: lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true),
	3: lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
	4: mutedStyle,
}

func Run() (*ExportedTask, error) {
	path, err := todo.DefaultPath()
	if err != nil {
		return nil, err
	}
	store, err := todo.Load(path)
	if err != nil {
		return nil, err
	}
	if len(store.Tasks) == 0 {
		store.Add("Press n to add your first task", "Inbox")
	}
	m := initialModel(store, path)
	final, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion()).Run()
	if err != nil {
		return nil, err
	}
	finished, ok := final.(model)
	if !ok || finished.exportID == 0 {
		return nil, nil
	}
	title := finished.exportTitle
	if title == "" {
		task, ok := finished.store.Task(finished.exportID)
		if !ok {
			return nil, nil
		}
		title = task.Title
	}
	return &ExportedTask{ID: finished.exportID, Title: title, Copied: finished.copyOnExit, Count: finished.exportCount()}, nil
}

func initialModel(store todo.Store, path string) model {
	view := "All"
	if views := visibleViews(store.Tasks); len(views) > 0 {
		view = views[0]
	}
	return model{
		store:      store,
		path:       path,
		view:       view,
		focus:      paneSidebar,
		selectFrom: -1,
		width:      100,
		height:     30,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case savedMsg:
		if msg.err != nil {
			m.message = msg.err.Error()
		} else {
			m.message = msg.text
		}
		m.messageAt = time.Now()
		return m, nil
	case copiedMsg:
		if msg.err != nil {
			m.flash("Copy failed: " + msg.err.Error())
		} else {
			m.flash("Copied")
		}
		return m, nil
	case tea.KeyMsg:
		if m.input.active {
			return m.updateInput(msg)
		}
		return m.updateNormal(msg)
	case tea.MouseMsg:
		if m.input.active {
			m.updateInputMouse(msg)
		}
		return m, nil
	}
	return m, nil
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editing {
		return m.updateEdit(msg)
	}
	if msg.String() != "D" {
		m.confirmDel = false
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Sequence(m.save("Saved"), tea.Quit)
	case "ctrl+z", "u":
		if m.undoStore != nil {
			m.store = *m.undoStore
			m.undoStore = nil
			m.clampSelection()
			return m, m.save("Undone")
		}
		m.flash("Nothing to undo")
	case "w":
		tasks := m.selectedTasks()
		if len(tasks) > 0 {
			m.exportID = tasks[0].ID
			m.exportTitle = taskTitles(tasks)
			return m, tea.Sequence(m.save("Saved"), tea.Quit)
		}
	case "W":
		tasks := m.selectedTasks()
		if len(tasks) > 0 {
			title := taskTitles(tasks)
			m.exportID = tasks[0].ID
			m.exportTitle = title
			m.copyOnExit = true
			return m, tea.Sequence(copyTaskCmd(title), m.save("Saved"), tea.Quit)
		}
	case "y":
		tasks := m.selectedTasks()
		if len(tasks) > 0 {
			return m, copyTaskCmd(taskTitles(tasks))
		}
	case "up", "k":
		m.clearTaskSelection()
		if m.focus == paneSidebar {
			m.moveSidebar(-1)
		} else {
			m.moveTask(-1)
		}
	case "down", "j":
		m.clearTaskSelection()
		if m.focus == paneSidebar {
			m.moveSidebar(1)
		} else {
			m.moveTask(1)
		}
	case "ctrl+up":
		if m.focus == paneTasks {
			m.extendTaskSelection(-1)
		}
	case "ctrl+down":
		if m.focus == paneTasks {
			m.extendTaskSelection(1)
		}
	case "left", "h":
		m.clearTaskSelection()
		m.focus = paneSidebar
	case "right", "l":
		m.focus = paneTasks
	case "tab":
		m.toggleFocus()
	case "n":
		m.startInput("new", "New task", "")
	case "e", "enter":
		if len(m.selectedTasks()) > 1 {
			m.flash("Cannot edit multiple tasks")
			break
		}
		if task := m.currentTask(); task != nil {
			m.editing = true
			m.editTaskID = task.ID
			m.editField = 0
			m.focus = paneTasks
			m.startInput("edit", "Edit", taskEditText(*task))
		}
	case "x", " ":
		if task := m.currentTask(); task != nil {
			m.checkpoint()
			task.ToggleComplete()
			return m, m.save("Task updated")
		}
	case "d":
		if task := m.currentTask(); task != nil {
			m.startInput("due", "Due date (today, tomorrow, +3d, yyyy-mm-dd, clear)", task.Due)
		}
	case "p":
		if task := m.currentTask(); task != nil {
			m.checkpoint()
			cyclePriority(task)
			return m, m.save("Priority updated")
		}
	case "P":
		if task := m.currentTask(); task != nil {
			m.startInput("project", "Project", task.Project)
		}
	case "L":
		if task := m.currentTask(); task != nil {
			m.startInput("labels", "Labels", strings.Join(task.Labels, ", "))
		}
	case "/":
		m.clearTaskSelection()
		m.startInput("search", "Search", "")
	case "c":
		m.search = ""
		m.flash("Search cleared")
	case "D":
		if task := m.currentTask(); task != nil {
			if !m.confirmDel {
				m.confirmDel = true
				m.flash("Press D again to delete")
				return m, nil
			}
			m.checkpoint()
			m.store.Delete(task.ID)
			m.confirmDel = false
			m.clampSelection()
			return m, m.save("Task deleted")
		}
	case "?":
		m.startInput("help", "Keys: left/right side, up/down move, n add, e edit, w export, W copy+quit, y copy, x done, / search, D delete, q quit", "")
	}
	m.clampSelection()
	return m, nil
}

func (m model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editingTask() == nil {
		m.editing = false
		m.editTaskID = 0
		return m, nil
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Sequence(m.save("Saved"), tea.Quit)
	case "esc", "e", "up", "down":
		m.editing = false
		m.editTaskID = 0
	case "left", "h":
		m.moveEditField(-1)
	case "right", "l", "tab":
		m.moveEditField(1)
	case "enter":
		return m.commitEditField()
	case "x":
		if task := m.editingTask(); task != nil {
			m.checkpoint()
			task.ToggleComplete()
			m.clampSelection()
			return m, m.save("Task updated")
		}
	case "p":
		if task := m.editingTask(); task != nil {
			m.checkpoint()
			cyclePriority(task)
			return m, m.save("Priority updated")
		}
	}
	return m, nil
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.input = inputState{}
		if m.editing {
			m.editing = false
			m.editTaskID = 0
		}
	case "enter":
		return m.commitInput()
	case "ctrl+z", "u":
		if len(m.inputUndo) > 0 {
			last := len(m.inputUndo) - 1
			m.input = m.inputUndo[last]
			m.inputUndo = m.inputUndo[:last]
			m.syncLiveInput()
		} else {
			m.flash("Nothing to undo")
		}
	case "up", "down":
		if m.input.kind == "edit" {
			m.input = inputState{}
			m.editing = false
			m.editTaskID = 0
		}
	case "left", "ctrl+b":
		m.clearInputSelection()
		m.moveInputCursor(-1)
	case "right", "ctrl+f":
		m.clearInputSelection()
		m.moveInputCursor(1)
	case "ctrl+left", "alt+left":
		m.clearInputSelection()
		m.input.cursor = previousWordStart([]rune(m.input.value), m.input.cursor)
	case "ctrl+right", "alt+right":
		m.clearInputSelection()
		m.input.cursor = nextWordEnd([]rune(m.input.value), m.input.cursor)
	case "home", "ctrl+a":
		m.clearInputSelection()
		m.input.cursor = 0
	case "end", "ctrl+e":
		m.clearInputSelection()
		m.input.cursor = len([]rune(m.input.value))
	case "delete":
		m.checkpointInput()
		m.deleteInputForward(false)
		m.syncLiveInput()
	case "alt+delete", "alt+d":
		m.checkpointInput()
		m.deleteInputForward(true)
		m.syncLiveInput()
	case "backspace":
		m.checkpointInput()
		m.deleteInputBackward(false)
		m.syncLiveInput()
	case "ctrl+h":
		m.checkpointInput()
		m.deleteInputBackward(false)
		m.syncLiveInput()
	case "ctrl+w", "alt+backspace":
		m.checkpointInput()
		m.deleteInputBackward(true)
		m.syncLiveInput()
	case "ctrl+u":
		m.checkpointInput()
		m.deleteInputBeforeCursor()
		m.syncLiveInput()
	case "ctrl+k":
		m.checkpointInput()
		m.deleteInputAfterCursor()
		m.syncLiveInput()
	case " ":
		m.checkpointInput()
		m.insertInputText(" ")
		m.syncLiveInput()
	default:
		if len(msg.Runes) > 0 {
			m.checkpointInput()
			m.insertInputText(string(msg.Runes))
			m.syncLiveInput()
		}
	}
	return m, nil
}

func (m *model) checkpointInput() {
	if !m.input.active {
		return
	}
	input := m.input
	if len(m.inputUndo) > 0 {
		last := m.inputUndo[len(m.inputUndo)-1]
		if last.value == input.value && last.cursor == input.cursor {
			return
		}
	}
	m.inputUndo = append(m.inputUndo, input)
}

func (m *model) updateInputMouse(msg tea.MouseMsg) {
	event := tea.MouseEvent(msg)
	if event.Button != tea.MouseButtonLeft {
		return
	}
	inputTop, inputHeight, available := m.inputLayout()
	if event.Y < inputTop || event.Y >= inputTop+inputHeight {
		return
	}
	prefixWidth := ansi.StringWidth(m.input.title + ": ")
	line := event.Y - inputTop
	x := event.X
	x -= prefixWidth
	nextCursor := 0
	if x > 0 {
		nextCursor = cursorIndexForInputPosition(m.input.value, available, line, x)
	}
	switch event.Action {
	case tea.MouseActionPress:
		m.input.cursor = nextCursor
		m.input.selectStart = nextCursor
		m.input.selecting = true
	case tea.MouseActionMotion:
		if !m.input.selecting {
			m.input.selectStart = m.input.cursor
			m.input.selecting = true
		}
		m.input.cursor = nextCursor
	}
}

func (m *model) insertInputText(text string) {
	m.deleteInputSelection()
	runes := []rune(m.input.value)
	m.clampInputCursor()
	insert := []rune(text)
	out := make([]rune, 0, len(runes)+len(insert))
	out = append(out, runes[:m.input.cursor]...)
	out = append(out, insert...)
	out = append(out, runes[m.input.cursor:]...)
	m.input.value = string(out)
	m.input.cursor += len(insert)
}

func (m *model) moveInputCursor(delta int) {
	m.input.cursor += delta
	m.clampInputCursor()
}

func (m *model) clampInputCursor() {
	length := len([]rune(m.input.value))
	if m.input.cursor < 0 {
		m.input.cursor = 0
	}
	if m.input.cursor > length {
		m.input.cursor = length
	}
}

func (m *model) deleteInputBackward(word bool) {
	if m.deleteInputSelection() {
		return
	}
	runes := []rune(m.input.value)
	m.clampInputCursor()
	if m.input.cursor == 0 {
		return
	}
	start := m.input.cursor - 1
	if word {
		start = previousWordStart(runes, m.input.cursor)
	}
	m.input.value = string(append(runes[:start], runes[m.input.cursor:]...))
	m.input.cursor = start
}

func (m *model) deleteInputForward(word bool) {
	if m.deleteInputSelection() {
		return
	}
	runes := []rune(m.input.value)
	m.clampInputCursor()
	if m.input.cursor >= len(runes) {
		return
	}
	end := m.input.cursor + 1
	if word {
		end = nextWordEnd(runes, m.input.cursor)
	}
	m.input.value = string(append(runes[:m.input.cursor], runes[end:]...))
}

func (m *model) deleteInputBeforeCursor() {
	if m.deleteInputSelection() {
		return
	}
	runes := []rune(m.input.value)
	m.clampInputCursor()
	m.input.value = string(runes[m.input.cursor:])
	m.input.cursor = 0
}

func (m *model) deleteInputAfterCursor() {
	if m.deleteInputSelection() {
		return
	}
	runes := []rune(m.input.value)
	m.clampInputCursor()
	m.input.value = string(runes[:m.input.cursor])
}

func (m *model) deleteInputSelection() bool {
	start, end, ok := m.inputSelectionBounds()
	if !ok {
		return false
	}
	runes := []rune(m.input.value)
	m.input.value = string(append(runes[:start], runes[end:]...))
	m.input.cursor = start
	m.clearInputSelection()
	return true
}

func (m *model) clearInputSelection() {
	m.input.selectStart = -1
	m.input.selecting = false
}

func (m model) inputSelectionBounds() (int, int, bool) {
	if !m.input.selecting || m.input.selectStart < 0 || m.input.selectStart == m.input.cursor {
		return 0, 0, false
	}
	start := min(m.input.selectStart, m.input.cursor)
	end := max(m.input.selectStart, m.input.cursor)
	length := len([]rune(m.input.value))
	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}
	return start, end, start < end
}

func (m *model) syncLiveInput() {
	if m.input.kind == "search" {
		m.search = strings.TrimSpace(m.input.value)
		m.clampSelection()
	}
}

func (m model) commitInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.value)
	kind := m.input.kind
	m.input = inputState{}
	switch kind {
	case "new":
		if value == "" {
			return m, nil
		}
		project := "Inbox"
		if strings.HasPrefix(m.view, "#") {
			project = strings.TrimPrefix(m.view, "#")
		}
		m.checkpoint()
		id := m.store.Add(value, project)
		m.selectID(id)
		return m, m.save("Task added")
	case "title":
		if task := m.targetTask(); task != nil && value != "" {
			m.checkpoint()
			if !todo.ApplyTaskText(task, value, time.Now()) {
				return m, nil
			}
			return m, m.save("Task updated")
		}
	case "edit":
		if task := m.editingTask(); task != nil && value != "" {
			m.checkpoint()
			if !todo.ReplaceTaskText(task, value, time.Now()) {
				return m, nil
			}
			m.editing = false
			m.editTaskID = 0
			m.clampSelection()
			return m, m.save("Task updated")
		}
	case "due":
		if task := m.targetTask(); task != nil {
			due, err := todo.NormalizeDue(value, time.Now())
			if err != nil {
				m.flash("Invalid due date")
				return m, nil
			}
			m.checkpoint()
			task.Due = due
			return m, m.save("Due date updated")
		}
	case "project":
		if task := m.targetTask(); task != nil {
			project := todo.CleanProject(value)
			if project == "" {
				project = "Inbox"
			}
			m.checkpoint()
			task.Project = project
			return m, m.save("Project updated")
		}
	case "labels":
		if task := m.targetTask(); task != nil {
			m.checkpoint()
			task.Labels = todo.CleanLabels(value)
			return m, m.save("Labels updated")
		}
	case "search":
		m.search = value
		m.flash("Search updated")
	case "help":
	}
	return m, nil
}

func (m model) View() string {
	if m.width <= 0 {
		m.width = 100
	}
	if m.height <= 0 {
		m.height = 30
	}
	tasks := m.filteredTasks()
	views := m.views()
	sideWidth := 22
	if m.width < 90 {
		sideWidth = 18
	}
	bodyWidth := m.width - sideWidth - 3
	if bodyWidth < 40 {
		bodyWidth = 40
	}
	contentRows := m.height - 6
	if contentRows < 8 {
		contentRows = 8
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("todos"))
	b.WriteString(mutedStyle.Render("  " + m.view + "  "))
	b.WriteString(m.focusLabel())
	b.WriteString(mutedStyle.Render("  data: " + m.path))
	b.WriteByte('\n')
	b.WriteString(borderStyle.Render(strings.Repeat("-", max(1, m.width))))
	b.WriteByte('\n')

	taskLines := m.taskListLines(tasks, bodyWidth, max(1, contentRows-1))
	for i := 0; i < contentRows; i++ {
		left := ""
		if i == 0 {
			left = m.sidebarHeader(sideWidth)
		} else if viewIndex := i - 1; viewIndex < len(views) {
			left = m.sidebarRow(viewIndex, views[viewIndex], sideWidth)
		}
		right := ""
		if i == 0 {
			right = m.headerRow(tasks)
		} else if lineIndex := i - 1; lineIndex < len(taskLines) {
			right = taskLines[lineIndex]
		}
		b.WriteString(pad(left, sideWidth))
		b.WriteString(borderStyle.Render(" | "))
		b.WriteString(pad(right, bodyWidth))
		b.WriteByte('\n')
	}

	b.WriteString(borderStyle.Render(strings.Repeat("-", max(1, m.width))))
	b.WriteByte('\n')
	if m.input.active {
		b.WriteString(m.inputView(m.width))
	} else if m.editing {
		b.WriteString(m.editBar(bodyWidth))
	} else {
		b.WriteString(mutedStyle.Render("left/right side  up/down move  ctrl+up/down select  tab side  n add  e edit  y copy  w export  W copy+quit  x done  / search  D delete  q quit"))
		if m.search != "" {
			b.WriteString(accentStyle.Render("  search: " + m.search))
		}
	}
	if m.message != "" && time.Since(m.messageAt) < 4*time.Second {
		b.WriteByte('\n')
		b.WriteString(accentStyle.Render(m.message))
	}
	return b.String()
}

func (m model) inputView(width int) string {
	prefix := m.input.title + ": "
	available := width - ansi.StringWidth(prefix)
	if available < 10 {
		available = 10
	}
	lines := wrapTextPreserveWords(m.input.value, available)
	if len(lines) == 0 {
		return prefix + cursorStyle.Render(" ")
	}
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(m.renderInputSegment(lines[0], 0))
	indent := strings.Repeat(" ", ansi.StringWidth(prefix))
	offset := len([]rune(lines[0]))
	for _, line := range lines[1:] {
		if offset < len([]rune(m.input.value)) {
			offset++
		}
		b.WriteByte('\n')
		b.WriteString(indent)
		b.WriteString(m.renderInputSegment(line, offset))
		offset += len([]rune(line))
	}
	if m.input.cursor == len([]rune(m.input.value)) {
		b.WriteString(cursorStyle.Render(" "))
	}
	return b.String()
}

func (m model) renderInputSegment(segment string, offset int) string {
	runes := []rune(segment)
	start, end, hasSelection := m.inputSelectionBounds()
	tokenStyles := inputTokenStyles(m.input.value)
	var b strings.Builder
	for i, r := range runes {
		index := offset + i
		rendered := string(r)
		if style, ok := tokenStyles[index]; ok && m.inputTokenColorEnabled() {
			rendered = style.Render(rendered)
		}
		if hasSelection && index >= start && index < end {
			rendered = cursorStyle.Render(string(r))
		} else if index == m.input.cursor {
			rendered = cursorStyle.Render(string(r))
		}
		b.WriteString(rendered)
	}
	return b.String()
}

func (m model) inputTokenColorEnabled() bool {
	return m.input.kind == "edit" || m.input.kind == "new"
}

func (m model) inputLayout() (top int, height int, available int) {
	width := m.width
	if width <= 0 {
		width = 100
	}
	contentRows := m.height - 6
	if contentRows < 8 {
		contentRows = 8
	}
	top = contentRows + 3
	prefixWidth := ansi.StringWidth(m.input.title + ": ")
	available = width - prefixWidth
	if available < 10 {
		available = 10
	}
	height = len(wrapTextPreserveWords(m.input.value, available))
	if height == 0 {
		height = 1
	}
	return top, height, available
}

func (m *model) startInput(kind, title, value string) {
	m.input = inputState{active: true, kind: kind, title: title, value: value, cursor: len([]rune(value)), selectStart: -1}
}

func (m *model) flash(message string) {
	m.message = message
	m.messageAt = time.Now()
}

func (m *model) checkpoint() {
	store := m.store
	store.Tasks = append([]todo.Task(nil), m.store.Tasks...)
	for i := range store.Tasks {
		store.Tasks[i].Labels = append([]string(nil), m.store.Tasks[i].Labels...)
	}
	m.undoStore = &store
}

func (m model) focusLabel() string {
	if m.editing {
		return activeChipStyle.Render("editing")
	}
	if m.focus == paneSidebar {
		return activeChipStyle.Render("views")
	}
	return activeChipStyle.Render("tasks")
}

func (m model) save(message string) tea.Cmd {
	store := m.store
	path := m.path
	return func() tea.Msg {
		return savedMsg{text: message, err: todo.Save(path, store)}
	}
}

func copyTaskCmd(text string) tea.Cmd {
	return func() tea.Msg {
		if path, err := exec.LookPath("clip.exe"); err == nil {
			cmd := exec.Command(path)
			stdin, err := cmd.StdinPipe()
			if err != nil {
				return copiedMsg{err: err}
			}
			if err := cmd.Start(); err != nil {
				return copiedMsg{err: err}
			}
			if _, err := stdin.Write([]byte(text)); err != nil {
				_ = stdin.Close()
				_ = cmd.Wait()
				return copiedMsg{err: err}
			}
			if err := stdin.Close(); err != nil {
				_ = cmd.Wait()
				return copiedMsg{err: err}
			}
			if err := cmd.Wait(); err != nil {
				return copiedMsg{err: err}
			}
			return copiedMsg{}
		}
		fmt.Print(osc52.New(text).String())
		return copiedMsg{}
	}
}

func (m *model) moveTask(delta int) {
	m.selected += delta
	m.clampSelection()
}

func (m *model) extendTaskSelection(delta int) {
	m.clampSelection()
	if len(m.taskIDs) == 0 {
		m.clearTaskSelection()
		return
	}
	if m.selectFrom < 0 {
		m.selectFrom = m.selected
	}
	m.selected += delta
	m.clampSelection()
}

func (m *model) clearTaskSelection() {
	m.selectFrom = -1
}

func (m *model) moveSidebar(delta int) {
	views := m.views()
	m.sidebar += delta
	if m.sidebar < 0 {
		m.sidebar = len(views) - 1
	}
	if m.sidebar >= len(views) {
		m.sidebar = 0
	}
	m.view = views[m.sidebar]
	m.selected = 0
	m.clearTaskSelection()
}

func (m *model) toggleFocus() {
	if m.focus == paneSidebar {
		m.focus = paneTasks
		return
	}
	m.focus = paneSidebar
	m.clearTaskSelection()
}

func (m *model) moveEditField(delta int) {
	m.editField += delta
	if m.editField < 0 {
		m.editField = len(editFields) - 1
	}
	if m.editField >= len(editFields) {
		m.editField = 0
	}
}

func (m model) commitEditField() (tea.Model, tea.Cmd) {
	task := m.editingTask()
	if task == nil {
		m.editing = false
		return m, nil
	}
	switch editFields[m.editField] {
	case "Title":
		m.startInput("title", "Title", task.Title)
	case "Project":
		m.startInput("project", "Project", task.Project)
	case "Due":
		m.startInput("due", "Due date (today, tomorrow, +3d, yyyy-mm-dd, clear)", task.Due)
	case "Priority":
		m.checkpoint()
		cyclePriority(task)
		return m, m.save("Priority updated")
	case "Labels":
		m.startInput("labels", "Labels", strings.Join(task.Labels, ", "))
	case "Completed":
		m.checkpoint()
		task.ToggleComplete()
		m.clampSelection()
		return m, m.save("Task updated")
	}
	return m, nil
}

func (m *model) clampSelection() {
	m.refreshTaskIDs()
	if len(m.taskIDs) == 0 {
		m.selected = 0
		m.clearTaskSelection()
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.taskIDs) {
		m.selected = len(m.taskIDs) - 1
	}
	if m.selectFrom >= len(m.taskIDs) {
		m.selectFrom = len(m.taskIDs) - 1
	}
}

func (m *model) refreshTaskIDs() []todo.Task {
	tasks := m.filteredTasks()
	m.taskIDs = m.taskIDs[:0]
	for _, task := range tasks {
		m.taskIDs = append(m.taskIDs, task.ID)
	}
	return tasks
}

func (m model) filteredTasks() []todo.Task {
	return todo.Filter(m.store.Tasks, m.view, m.search, time.Now())
}

func (m *model) currentTask() *todo.Task {
	m.clampSelection()
	if len(m.taskIDs) == 0 {
		return nil
	}
	task, ok := m.store.Task(m.taskIDs[m.selected])
	if !ok {
		return nil
	}
	return task
}

func (m *model) selectedTasks() []todo.Task {
	tasks := m.refreshTaskIDs()
	if len(tasks) == 0 {
		return nil
	}
	start, end := m.selectionBounds()
	if start < 0 || end < 0 {
		return nil
	}
	out := make([]todo.Task, 0, end-start+1)
	for i := start; i <= end && i < len(tasks); i++ {
		out = append(out, tasks[i])
	}
	return out
}

func (m model) selectionBounds() (int, int) {
	if len(m.taskIDs) == 0 {
		return -1, -1
	}
	start := m.selected
	end := m.selected
	if m.selectFrom >= 0 {
		start = min(m.selectFrom, m.selected)
		end = max(m.selectFrom, m.selected)
	}
	return start, end
}

func (m *model) targetTask() *todo.Task {
	if m.editing {
		return m.editingTask()
	}
	return m.currentTask()
}

func (m *model) editingTask() *todo.Task {
	if m.editTaskID == 0 {
		return nil
	}
	task, ok := m.store.Task(m.editTaskID)
	if !ok {
		return nil
	}
	return task
}

func (m *model) selectID(id int) {
	m.refreshTaskIDs()
	for i, taskID := range m.taskIDs {
		if taskID == id {
			m.selected = i
			return
		}
	}
}

func (m model) views() []string {
	return visibleViews(m.store.Tasks)
}

func visibleViews(tasks []todo.Task) []string {
	var views []string
	for _, view := range []string{"Today", "Upcoming", "All", "Completed"} {
		if len(todo.Filter(tasks, view, "", time.Now())) > 0 {
			views = append(views, view)
		}
	}
	for _, project := range todo.Projects(tasks) {
		view := "#" + project
		if project != "Inbox" && len(todo.Filter(tasks, view, "", time.Now())) > 0 {
			views = append(views, "#"+project)
		}
	}
	for _, label := range todo.Labels(tasks) {
		view := "@" + label
		if len(todo.Filter(tasks, view, "", time.Now())) > 0 {
			views = append(views, view)
		}
	}
	return views
}

func (m model) sidebarHeader(width int) string {
	label := "Views"
	if m.focus == paneSidebar {
		return sectionStyle.Render(pad(label, width))
	}
	return mutedStyle.Render(pad(label, width))
}

func (m model) sidebarRow(i int, view string, width int) string {
	count := len(todo.Filter(m.store.Tasks, view, "", time.Now()))
	label := fmt.Sprintf("%s %d", view, count)
	row := truncate(label, width)
	if i == m.sidebar {
		if m.focus != paneSidebar {
			return inactiveSelectedStyle.Render(pad(row, width))
		}
		return selectedStyle.Render(pad(row, width))
	}
	return row
}

func (m model) headerRow(tasks []todo.Task) string {
	open := 0
	for _, task := range m.store.Tasks {
		if !task.Completed {
			open++
		}
	}
	title := fmt.Sprintf("%d tasks", len(tasks))
	if selected := len(m.selectedTasks()); selected > 1 {
		title = fmt.Sprintf("%d selected", selected)
	}
	if m.focus == paneTasks {
		title = sectionStyle.Render(title)
	} else {
		title = mutedStyle.Render(title)
	}
	return fmt.Sprintf("%s %s", title, mutedStyle.Render(fmt.Sprintf("(%d open total)", open)))
}

func (m model) taskListLines(tasks []todo.Task, width int, visible int) []string {
	if visible <= 0 {
		return nil
	}
	if m.selectFrom >= 0 {
		return m.selectedTaskRangeBox(tasks, width, visible)
	}
	taskStart := scrollStart(m.selected, max(1, visible-4), len(tasks))
	lines := make([]string, 0, visible)
	for i := taskStart; i < len(tasks) && len(lines) < visible; i++ {
		if i == m.selected {
			box := m.selectedTaskBox(i, tasks[i], width)
			for _, line := range box {
				if len(lines) < visible {
					lines = append(lines, line)
				}
			}
			continue
		}
		line := m.taskRow(i, tasks[i], width)
		if m.taskIndexSelected(i) {
			line = inactiveSelectedStyle.Render(pad(line, width))
		}
		lines = append(lines, line)
	}
	return lines
}

func (m model) selectedTaskRangeBox(tasks []todo.Task, width int, visible int) []string {
	if width < 8 {
		return nil
	}
	start, end := m.selectionBounds()
	if start < 0 || end < 0 || start >= len(tasks) {
		return nil
	}
	if end >= len(tasks) {
		end = len(tasks) - 1
	}
	taskStart := scrollStart(m.selected, max(1, visible-2), len(tasks))
	if start < taskStart {
		taskStart = start
	}
	style := m.multiTaskBorderStyle()
	lines := make([]string, 0, visible)
	for i := taskStart; i < len(tasks) && len(lines) < visible; i++ {
		if i == start {
			lines = append(lines, style.Render("+"+strings.Repeat("-", width-2)+"+"))
		}
		if i >= start && i <= end {
			row := m.taskRow(i, tasks[i], width-4)
			lines = append(lines, style.Render("| ")+pad(row, width-4)+style.Render(" |"))
			if i == end && len(lines) < visible {
				lines = append(lines, style.Render("+"+strings.Repeat("-", width-2)+"+"))
			}
			continue
		}
		lines = append(lines, m.taskRow(i, tasks[i], width))
	}
	return lines
}

func (m model) taskIndexSelected(index int) bool {
	if m.selectFrom < 0 {
		return false
	}
	start, end := m.selectionBounds()
	return index >= start && index <= end
}

func (m model) selectedTaskBox(index int, task todo.Task, width int) []string {
	if width < 8 {
		return []string{m.taskRow(index, task, width)}
	}
	style := m.singleTaskBorderStyle(task)
	top := style.Render("+" + strings.Repeat("-", width-2) + "+")
	content := m.taskBoxContent(index, task, width-4)
	middle := make([]string, 0, len(content))
	for _, line := range content {
		middle = append(middle, style.Render("| ")+pad(line, width-4)+style.Render(" |"))
	}
	bottom := style.Render("+" + strings.Repeat("-", width-2) + "+")
	return append(append([]string{top}, middle...), bottom)
}

func (m model) singleTaskBorderStyle(task todo.Task) lipgloss.Style {
	if m.focus != paneTasks || m.editing {
		return inactiveBorderStyle
	}
	return priorityStyle(task.Priority)
}

func (m model) multiTaskBorderStyle() lipgloss.Style {
	if m.focus != paneTasks || m.editing {
		return inactiveBorderStyle
	}
	return multiBorderStyle
}

func (m model) taskBoxContent(index int, task todo.Task, width int) []string {
	if width < 10 {
		return []string{m.taskRow(index, task, width)}
	}
	check := " "
	if task.Completed {
		check = "x"
	}
	cursor := "  "
	if index == m.selected && m.focus == paneTasks {
		cursor = "> "
	}
	priority := priorityBadge(task.Priority, false)
	due := dueBadge(task, false)
	project := projectBadge(task.Project)
	labels := labelsBadge(task.Labels, false)
	prefix := fmt.Sprintf("%s[%s] %s ", cursor, check, priority)
	meta := strings.TrimSpace(strings.Join(nonEmptyStrings(due, project, labels), " "))
	titleWidth := width - ansi.StringWidth(prefix)
	if meta != "" {
		inlineTitleWidth := titleWidth - 1 - ansi.StringWidth(meta)
		if inlineTitleWidth >= 10 && ansi.StringWidth(task.Title) <= inlineTitleWidth {
			return []string{prefix + task.Title + " " + meta}
		}
	}
	if titleWidth < 10 {
		titleWidth = 10
	}
	lines := wrapText(task.Title, titleWidth)
	if len(lines) == 0 {
		lines = []string{""}
	}
	rows := make([]string, 0, len(lines)+1)
	rows = append(rows, prefix+lines[0])
	indent := strings.Repeat(" ", ansi.StringWidth(prefix))
	for _, line := range lines[1:] {
		rows = append(rows, indent+line)
	}
	if meta != "" {
		rows = append(rows, indent+meta)
	}
	return rows
}

func (m model) taskRow(index int, task todo.Task, width int) string {
	check := " "
	if task.Completed {
		check = "x"
	}
	cursor := "  "
	if index == m.selected && m.focus == paneTasks {
		cursor = "> "
	}
	priority := priorityBadge(task.Priority, false)
	due := dueBadge(task, false)
	project := projectBadge(task.Project)
	labels := labelsBadge(task.Labels, false)
	if index == m.selected {
		project = projectBadge(task.Project)
	}
	textBudget := width - ansi.StringWidth(cursor) - 4 - ansi.StringWidth(priority) - ansi.StringWidth(due) - ansi.StringWidth(project) - ansi.StringWidth(labels)
	if textBudget < 10 {
		textBudget = 10
	}
	title := truncate(task.Title, textBudget)
	row := fmt.Sprintf("%s[%s] %s %s %s %s%s", cursor, check, priority, title, due, project, labels)
	return row
}

var editFields = []string{"Title", "Project", "Due", "Priority", "Labels", "Completed"}

func taskEditText(task todo.Task) string {
	parts := []string{task.Title}
	if task.Priority != 4 {
		parts = append(parts, fmt.Sprintf("p%d", task.Priority))
	}
	if task.Due != "" {
		parts = append(parts, task.Due)
	}
	if task.Project != "" && task.Project != "Inbox" {
		parts = append(parts, "#"+task.Project)
	}
	for _, label := range task.Labels {
		parts = append(parts, "@"+label)
	}
	return strings.Join(parts, " ")
}

func taskTitles(tasks []todo.Task) string {
	parts := make([]string, 0, len(tasks))
	for _, task := range tasks {
		parts = append(parts, task.Title)
	}
	return strings.Join(parts, "\n")
}

func inputTokenStyles(value string) map[int]lipgloss.Style {
	styles := map[int]lipgloss.Style{}
	runes := []rune(value)
	start := -1
	applyToken := func(end int) {
		if start < 0 || start >= end {
			return
		}
		token := string(runes[start:end])
		style, ok := inputTokenStyle(token)
		if !ok {
			return
		}
		for i := start; i < end; i++ {
			styles[i] = style
		}
	}
	for i, r := range runes {
		if r == ' ' || r == '\t' {
			applyToken(i)
			start = -1
			continue
		}
		if start < 0 {
			start = i
		}
	}
	applyToken(len(runes))
	return styles
}

func inputTokenStyle(token string) (lipgloss.Style, bool) {
	switch {
	case isPriorityToken(token):
		return priorityStyle(int(token[1] - '0')), true
	case strings.HasPrefix(token, "#") && len(token) > 1:
		return projectInputStyle, true
	case strings.HasPrefix(token, "@") && len(token) > 1:
		return labelInputStyle, true
	case isDateToken(token):
		return dateTokenStyle(token, time.Now()), true
	default:
		return lipgloss.Style{}, false
	}
}

func dateTokenStyle(token string, now time.Time) lipgloss.Style {
	d, err := todo.ParseDue(token, now)
	if err != nil || d.IsZero() {
		return dateInputStyle
	}
	return dueStyleAt(d, now)
}

func isPriorityToken(token string) bool {
	return len(token) == 2 && token[0] == 'p' && token[1] >= '1' && token[1] <= '4'
}

func isDateToken(token string) bool {
	if token == "today" || token == "tomorrow" {
		return true
	}
	if strings.HasPrefix(token, "+") && strings.HasSuffix(token, "d") && len(token) > 2 {
		return true
	}
	if len(token) != len("2006-01-02") {
		return false
	}
	for i, r := range token {
		if i == 4 || i == 7 {
			if r != '-' {
				return false
			}
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (m model) exportCount() int {
	if m.exportTitle == "" {
		return 1
	}
	return strings.Count(m.exportTitle, "\n") + 1
}

func (m model) editBar(width int) string {
	task := m.editingTask()
	if task == nil {
		return warnStyle.Render("Task no longer exists")
	}
	values := []string{
		task.Title,
		task.Project,
		emptyAs(task.Due, "none"),
		fmt.Sprintf("p%d", task.Priority),
		emptyAs(strings.Join(task.Labels, ", "), "none"),
		fmt.Sprintf("%t", task.Completed),
	}
	var fields []string
	for i, field := range editFields {
		text := fmt.Sprintf("%s: %s", field, values[i])
		if i == m.editField {
			text = activeChipStyle.Render(text)
		} else {
			text = chipStyle.Render(text)
		}
		fields = append(fields, text)
	}
	line := "edit mode  " + strings.Join(fields, "  ")
	help := mutedStyle.Render("  left/right field  enter change  p priority  x complete  esc/e close")
	return truncate(line+help, width)
}

func dueBadge(task todo.Task, plain bool) string {
	return dueBadgeAt(task, plain, time.Now())
}

func dueBadgeAt(task todo.Task, plain bool, now time.Time) string {
	if task.Due == "" {
		if plain {
			return "no due"
		}
		return mutedStyle.Render("no due")
	}
	d, ok := task.DueTimeAt(now)
	if !ok {
		return task.Due
	}
	label := task.Due
	start := startOfDay(now)
	switch {
	case d.Before(start):
		label = task.Due
	case d.Equal(start):
		label = "today"
	case d.Equal(start.AddDate(0, 0, 1)):
		label = "tomorrow"
	default:
		label = task.Due
	}
	if plain {
		return label
	}
	return dueStyleAt(d, now).Render(label)
}

func dueStyleAt(d time.Time, now time.Time) lipgloss.Style {
	start := startOfDay(now)
	switch {
	case d.Before(start):
		return warnStyle
	case d.Equal(start):
		return todayDateStyle
	case d.Equal(start.AddDate(0, 0, 1)):
		return dateInputStyle
	default:
		return futureDateStyle
	}
}

func startOfDay(t time.Time) time.Time {
	y, month, day := t.Date()
	return time.Date(y, month, day, 0, 0, 0, 0, t.Location())
}

func priorityBadge(priority int, plain bool) string {
	label := fmt.Sprintf("p%d", priority)
	if plain {
		return label
	}
	return priorityStyle(priority).Render(label)
}

func priorityStyle(priority int) lipgloss.Style {
	if style, ok := priorityStyles[priority]; ok {
		return style
	}
	return mutedStyle
}

func highestPriority(tasks []todo.Task) int {
	priority := 4
	for _, task := range tasks {
		if task.Priority < priority {
			priority = task.Priority
		}
	}
	return priority
}

func projectBadge(project string) string {
	if project == "" || project == "Inbox" {
		return ""
	}
	value := "#" + project
	return projectInputStyle.Render(value)
}

func labelsBadge(labels []string, plain bool) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, len(labels))
	for i, label := range labels {
		parts[i] = "@" + label
	}
	value := " " + strings.Join(parts, " ")
	if plain {
		return value
	}
	return labelInputStyle.Render(value)
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func wrapText(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	line := ""
	for _, word := range words {
		for ansi.StringWidth(word) > width {
			head := ansi.Truncate(word, width, "")
			if head == "" {
				break
			}
			lines = appendPendingLine(lines, line)
			line = ""
			lines = append(lines, head)
			word = strings.TrimPrefix(word, head)
		}
		if line == "" {
			line = word
			continue
		}
		if ansi.StringWidth(line)+1+ansi.StringWidth(word) <= width {
			line += " " + word
			continue
		}
		lines = append(lines, line)
		line = word
	}
	return appendPendingLine(lines, line)
}

func wrapTextPreserveWords(s string, width int) []string {
	if s == "" {
		return nil
	}
	return wrapInputText(s, width)
}

func wrapInputText(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return nil
	}
	var lines []string
	lineStart := 0
	lineWidth := 0
	for i, r := range runes {
		rw := ansi.StringWidth(string(r))
		if lineWidth > 0 && lineWidth+rw > width {
			lines = append(lines, string(runes[lineStart:i]))
			lineStart = i
			lineWidth = 0
		}
		lineWidth += rw
	}
	lines = append(lines, string(runes[lineStart:]))
	return lines
}

func inputValueWithCursor(value string, cursor int) string {
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	if cursor == len(runes) {
		return string(runes) + cursorStyle.Render(" ")
	}
	return string(runes[:cursor]) + cursorStyle.Render(string(runes[cursor])) + string(runes[cursor+1:])
}

func previousWordStart(runes []rune, cursor int) int {
	i := cursor
	for i > 0 && runes[i-1] == ' ' {
		i--
	}
	for i > 0 && runes[i-1] != ' ' {
		i--
	}
	return i
}

func nextWordEnd(runes []rune, cursor int) int {
	i := cursor
	for i < len(runes) && runes[i] == ' ' {
		i++
	}
	for i < len(runes) && runes[i] != ' ' {
		i++
	}
	for i < len(runes) && runes[i] == ' ' {
		i++
	}
	return i
}

func cursorIndexForWidth(value string, width int) int {
	if width <= 0 {
		return 0
	}
	current := 0
	for i, r := range []rune(value) {
		next := current + ansi.StringWidth(string(r))
		if width < next {
			return i
		}
		current = next
	}
	return len([]rune(value))
}

func cursorIndexForInputPosition(value string, width int, line int, x int) int {
	if line <= 0 {
		return cursorIndexForWidth(value, x)
	}
	lines := wrapTextPreserveWords(value, width)
	if len(lines) == 0 {
		return 0
	}
	if line >= len(lines) {
		return len([]rune(value))
	}
	offset := 0
	for i := 0; i < line; i++ {
		offset += len([]rune(lines[i]))
	}
	return min(len([]rune(value)), offset+cursorIndexForWidth(lines[line], x))
}

func appendPendingLine(lines []string, line string) []string {
	if line == "" {
		return lines
	}
	return append(lines, line)
}

func cyclePriority(task *todo.Task) {
	task.Priority--
	if task.Priority < 1 {
		task.Priority = 4
	}
}

func emptyAs(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func pad(s string, width int) string {
	if ansi.StringWidth(s) >= width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-ansi.StringWidth(s))
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, ".")
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

func scrollStart(selected, visible, total int) int {
	if visible <= 0 || total <= visible || selected < visible {
		return 0
	}
	start := selected - visible + 1
	if start+visible > total {
		return total - visible
	}
	return start
}
