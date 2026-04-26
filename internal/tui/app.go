package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"todos/internal/todo"
)

type model struct {
	store      todo.Store
	path       string
	view       string
	focus      pane
	sidebar    int
	selected   int
	taskIDs    []int
	width      int
	height     int
	search     string
	input      inputState
	editing    bool
	editTaskID int
	editField  int
	message    string
	messageAt  time.Time
	confirmDel bool
	exportID   int
	exportPlan bool
}

type ExportedTask struct {
	ID      int
	Title   string
	RunPlan bool
}

type pane int

const (
	paneSidebar pane = iota
	paneTasks
)

type inputState struct {
	active bool
	kind   string
	title  string
	value  string
	cursor int
}

type savedMsg struct {
	text string
	err  error
}

var (
	titleStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	sectionStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	mutedStyle            = lipgloss.NewStyle().Faint(true)
	accentStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	warnStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	selectedStyle         = lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.Color("15")).Bold(true)
	inactiveSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	selectedBorderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	inactiveBorderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	borderStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	chipStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8")).Padding(0, 1)
	activeChipStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("9")).Bold(true).Padding(0, 1)
	cursorStyle           = lipgloss.NewStyle().Reverse(true)
)

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
	m := model{
		store:  store,
		path:   path,
		view:   "Inbox",
		focus:  paneTasks,
		width:  100,
		height: 30,
	}
	final, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion()).Run()
	if err != nil {
		return nil, err
	}
	finished, ok := final.(model)
	if !ok || finished.exportID == 0 {
		return nil, nil
	}
	task, ok := finished.store.Task(finished.exportID)
	if !ok {
		return nil, nil
	}
	return &ExportedTask{ID: task.ID, Title: task.Title, RunPlan: finished.exportPlan}, nil
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
	if m.input.active && isCtrlDeleteSequence(msg) {
		m.deleteInputForward(true)
		m.syncLiveInput()
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
	case "w":
		if task := m.currentTask(); task != nil {
			m.exportID = task.ID
			return m, tea.Sequence(m.save("Saved"), tea.Quit)
		}
	case "W":
		if task := m.currentTask(); task != nil {
			m.exportID = task.ID
			m.exportPlan = true
			return m, tea.Sequence(m.save("Saved"), tea.Quit)
		}
	case "up", "k":
		if m.focus == paneSidebar {
			m.moveSidebar(-1)
		} else {
			m.moveTask(-1)
		}
	case "down", "j":
		if m.focus == paneSidebar {
			m.moveSidebar(1)
		} else {
			m.moveTask(1)
		}
	case "left", "h":
		m.focus = paneSidebar
	case "right", "l":
		m.focus = paneTasks
	case "tab":
		m.toggleFocus()
	case "n":
		m.startInput("new", "New task", "")
	case "e", "enter":
		if task := m.currentTask(); task != nil {
			m.editing = true
			m.editTaskID = task.ID
			m.editField = 0
			m.focus = paneTasks
			m.startInput("edit", "Edit", taskEditText(*task))
		}
	case "x", " ":
		if task := m.currentTask(); task != nil {
			task.ToggleComplete()
			return m, m.save("Task updated")
		}
	case "d":
		if task := m.currentTask(); task != nil {
			m.startInput("due", "Due date (today, tomorrow, +3d, yyyy-mm-dd, clear)", task.Due)
		}
	case "p":
		if task := m.currentTask(); task != nil {
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
			m.store.Delete(task.ID)
			m.confirmDel = false
			m.clampSelection()
			return m, m.save("Task deleted")
		}
	case "?":
		m.startInput("help", "Keys: left/right side, up/down move, n add, e edit, w export, W plan, x done, / search, D delete, q quit", "")
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
			task.ToggleComplete()
			m.clampSelection()
			return m, m.save("Task updated")
		}
	case "p":
		if task := m.editingTask(); task != nil {
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
	case "left", "ctrl+b":
		m.moveInputCursor(-1)
	case "right", "ctrl+f":
		m.moveInputCursor(1)
	case "ctrl+left", "alt+left":
		m.input.cursor = previousWordStart([]rune(m.input.value), m.input.cursor)
	case "ctrl+right", "alt+right":
		m.input.cursor = nextWordEnd([]rune(m.input.value), m.input.cursor)
	case "home", "ctrl+a":
		m.input.cursor = 0
	case "end", "ctrl+e":
		m.input.cursor = len([]rune(m.input.value))
	case "delete":
		m.deleteInputForward(false)
		m.syncLiveInput()
	case "ctrl+delete", "alt+delete", "alt+d":
		m.deleteInputForward(true)
		m.syncLiveInput()
	case "backspace":
		m.deleteInputBackward(false)
		m.syncLiveInput()
	case "ctrl+h":
		m.deleteInputBackward(false)
		m.syncLiveInput()
	case "ctrl+w", "alt+backspace":
		m.deleteInputBackward(true)
		m.syncLiveInput()
	case "ctrl+u":
		m.deleteInputBeforeCursor()
		m.syncLiveInput()
	case "ctrl+k":
		m.deleteInputAfterCursor()
		m.syncLiveInput()
	default:
		if len(msg.Runes) > 0 {
			m.insertInputText(string(msg.Runes))
			m.syncLiveInput()
		}
	}
	return m, nil
}

func (m *model) updateInputMouse(msg tea.MouseMsg) {
	event := tea.MouseEvent(msg)
	if event.Action != tea.MouseActionPress || event.Button != tea.MouseButtonLeft {
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
	if x <= 0 {
		m.input.cursor = 0
		return
	}
	m.input.cursor = cursorIndexForInputPosition(m.input.value, available, line, x)
}

func (m *model) insertInputText(text string) {
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
	runes := []rune(m.input.value)
	m.clampInputCursor()
	m.input.value = string(runes[m.input.cursor:])
	m.input.cursor = 0
}

func (m *model) deleteInputAfterCursor() {
	runes := []rune(m.input.value)
	m.clampInputCursor()
	m.input.value = string(runes[:m.input.cursor])
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
		id := m.store.Add(value, project)
		m.selectID(id)
		return m, m.save("Task added")
	case "title":
		if task := m.targetTask(); task != nil && value != "" {
			if !todo.ApplyTaskText(task, value, time.Now()) {
				return m, nil
			}
			return m, m.save("Task updated")
		}
	case "edit":
		if task := m.editingTask(); task != nil && value != "" {
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
			task.Due = due
			return m, m.save("Due date updated")
		}
	case "project":
		if task := m.targetTask(); task != nil {
			project := todo.CleanProject(value)
			if project == "" {
				project = "Inbox"
			}
			task.Project = project
			return m, m.save("Project updated")
		}
	case "labels":
		if task := m.targetTask(); task != nil {
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
	contentRows := m.height - 5
	if contentRows < 8 {
		contentRows = 8
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("todos"))
	b.WriteString(mutedStyle.Render("  " + m.view + "  "))
	b.WriteString(m.focusLabel())
	b.WriteString(mutedStyle.Render("  data: " + m.path))
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
		b.WriteString(mutedStyle.Render("left/right side  up/down move  tab side  n add  e edit  w export  W plan  x done  / search  D delete  q quit"))
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
	value := inputValueWithCursor(m.input.value, m.input.cursor)
	lines := wrapTextPreserveWords(value, available)
	if len(lines) == 0 {
		return prefix + cursorStyle.Render(" ")
	}
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(lines[0])
	indent := strings.Repeat(" ", ansi.StringWidth(prefix))
	for _, line := range lines[1:] {
		b.WriteByte('\n')
		b.WriteString(indent)
		b.WriteString(line)
	}
	return b.String()
}

func (m model) inputLayout() (top int, height int, available int) {
	width := m.width
	if width <= 0 {
		width = 100
	}
	contentRows := m.height - 5
	if contentRows < 8 {
		contentRows = 8
	}
	top = contentRows + 2
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
	m.input = inputState{active: true, kind: kind, title: title, value: value, cursor: len([]rune(value))}
}

func (m *model) flash(message string) {
	m.message = message
	m.messageAt = time.Now()
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

func (m *model) moveTask(delta int) {
	m.selected += delta
	m.clampSelection()
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
}

func (m *model) toggleFocus() {
	if m.focus == paneSidebar {
		m.focus = paneTasks
		return
	}
	m.focus = paneSidebar
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
		cyclePriority(task)
		return m, m.save("Priority updated")
	case "Labels":
		m.startInput("labels", "Labels", strings.Join(task.Labels, ", "))
	case "Completed":
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
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.taskIDs) {
		m.selected = len(m.taskIDs) - 1
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
	views := []string{"Inbox", "Today", "Upcoming", "All", "Completed"}
	for _, project := range todo.Projects(m.store.Tasks) {
		if project != "Inbox" {
			views = append(views, "#"+project)
		}
	}
	for _, label := range todo.Labels(m.store.Tasks) {
		views = append(views, "@"+label)
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
		lines = append(lines, m.taskRow(i, tasks[i], width))
	}
	return lines
}

func (m model) selectedTaskBox(index int, task todo.Task, width int) []string {
	if width < 8 {
		return []string{m.taskRow(index, task, width)}
	}
	style := selectedBorderStyle
	if m.focus != paneTasks || m.editing {
		style = inactiveBorderStyle
	}
	top := style.Render("+" + strings.Repeat("-", width-2) + "+")
	content := m.taskBoxContent(index, task, width-4)
	middle := make([]string, 0, len(content))
	for _, line := range content {
		middle = append(middle, style.Render("| ")+pad(line, width-4)+style.Render(" |"))
	}
	bottom := style.Render("+" + strings.Repeat("-", width-2) + "+")
	return append(append([]string{top}, middle...), bottom)
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
	if task.Due == "" {
		if plain {
			return "no due"
		}
		return mutedStyle.Render("no due")
	}
	d, ok := task.DueTime()
	if !ok {
		return task.Due
	}
	today := time.Now()
	y, month, day := today.Date()
	start := time.Date(y, month, day, 0, 0, 0, 0, today.Location())
	switch {
	case d.Before(start):
		if plain {
			return task.Due
		}
		return warnStyle.Render(task.Due)
	case d.Equal(start):
		if plain {
			return "today"
		}
		return accentStyle.Render("today")
	case d.Equal(start.AddDate(0, 0, 1)):
		return "tomorrow"
	default:
		return task.Due
	}
}

func priorityBadge(priority int, plain bool) string {
	label := fmt.Sprintf("p%d", priority)
	if plain {
		return label
	}
	switch priority {
	case 1:
		return warnStyle.Render(label)
	case 2:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).Render(label)
	case 3:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Render(label)
	default:
		return mutedStyle.Render(label)
	}
}

func projectBadge(project string) string {
	if project == "" || project == "Inbox" {
		return ""
	}
	value := "#" + project
	return accentStyle.Render(value)
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
	return mutedStyle.Render(value)
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
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return wrapText(s, width)
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
		if offset < len([]rune(value)) {
			offset++
		}
	}
	return min(len([]rune(value)), offset+cursorIndexForWidth(lines[line], x))
}

func isCtrlDeleteSequence(msg tea.Msg) bool {
	stringer, ok := msg.(fmt.Stringer)
	if !ok {
		return false
	}
	switch stringer.String() {
	case "?CSI[51 59 53 126]?", "?CSI[51 59 54 126]?", "?CSI[51 59 55 126]?", "?CSI[51 59 56 126]?":
		return true
	default:
		return false
	}
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
