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
	sidebar    int
	selected   int
	taskIDs    []int
	width      int
	height     int
	search     string
	input      inputState
	message    string
	messageAt  time.Time
	confirmDel bool
}

type inputState struct {
	active bool
	kind   string
	title  string
	value  string
}

type savedMsg struct {
	text string
	err  error
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	mutedStyle    = lipgloss.NewStyle().Faint(true)
	accentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	selectedStyle = lipgloss.NewStyle().Reverse(true)
	borderStyle   = lipgloss.NewStyle().Faint(true)
)

func Run() error {
	path, err := todo.DefaultPath()
	if err != nil {
		return err
	}
	store, err := todo.Load(path)
	if err != nil {
		return err
	}
	if len(store.Tasks) == 0 {
		store.Add("Press n to add your first task", "Inbox")
	}
	m := model{
		store:  store,
		path:   path,
		view:   "Inbox",
		width:  100,
		height: 30,
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
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
	}
	return m, nil
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "D" {
		m.confirmDel = false
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Sequence(m.save("Saved"), tea.Quit)
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "left", "h":
		m.moveSidebar(-1)
	case "right", "l", "tab":
		m.moveSidebar(1)
	case "n":
		m.startInput("new", "New task", "")
	case "e", "enter":
		if task := m.currentTask(); task != nil {
			m.startInput("title", "Edit title", task.Title)
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
			task.Priority--
			if task.Priority < 1 {
				task.Priority = 4
			}
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
		m.startInput("search", "Search", m.search)
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
		m.startInput("help", "Keys: n add, e edit, x done, d due, p priority, P project, L labels, / search, D delete, q quit", "")
	}
	m.clampSelection()
	return m, nil
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.input = inputState{}
	case "enter":
		return m.commitInput()
	case "backspace":
		if len(m.input.value) > 0 {
			runes := []rune(m.input.value)
			m.input.value = string(runes[:len(runes)-1])
		}
	default:
		if len(msg.Runes) > 0 {
			m.input.value += string(msg.Runes)
		}
	}
	return m, nil
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
		if task := m.currentTask(); task != nil && value != "" {
			task.Title = value
			return m, m.save("Task updated")
		}
	case "due":
		if task := m.currentTask(); task != nil {
			due, err := todo.NormalizeDue(value, time.Now())
			if err != nil {
				m.flash("Invalid due date")
				return m, nil
			}
			task.Due = due
			return m, m.save("Due date updated")
		}
	case "project":
		if task := m.currentTask(); task != nil {
			project := todo.CleanProject(value)
			if project == "" {
				project = "Inbox"
			}
			task.Project = project
			return m, m.save("Project updated")
		}
	case "labels":
		if task := m.currentTask(); task != nil {
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
	b.WriteString(mutedStyle.Render(fmt.Sprintf("  %s  data: %s", m.view, m.path)))
	b.WriteByte('\n')

	taskStart := scrollStart(m.selected, max(1, contentRows-1), len(tasks))
	for i := 0; i < contentRows; i++ {
		left := ""
		if i < len(views) {
			left = m.sidebarRow(i, views[i], sideWidth)
		}
		right := ""
		if i == 0 {
			right = m.headerRow(tasks)
		} else if taskIndex := taskStart + i - 1; taskIndex < len(tasks) {
			right = m.taskRow(taskIndex, tasks[taskIndex], bodyWidth)
		}
		b.WriteString(pad(left, sideWidth))
		b.WriteString(borderStyle.Render(" | "))
		b.WriteString(pad(right, bodyWidth))
		b.WriteByte('\n')
	}

	b.WriteString(borderStyle.Render(strings.Repeat("-", max(1, m.width))))
	b.WriteByte('\n')
	if m.input.active {
		b.WriteString(m.input.title)
		b.WriteString(": ")
		b.WriteString(m.input.value)
	} else {
		b.WriteString(mutedStyle.Render("j/k move  tab view  n add  e edit  x done  d due  p priority  P project  L labels  / search  D delete  q quit"))
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

func (m *model) startInput(kind, title, value string) {
	m.input = inputState{active: true, kind: kind, title: title, value: value}
}

func (m *model) flash(message string) {
	m.message = message
	m.messageAt = time.Now()
}

func (m model) save(message string) tea.Cmd {
	store := m.store
	path := m.path
	return func() tea.Msg {
		return savedMsg{text: message, err: todo.Save(path, store)}
	}
}

func (m *model) move(delta int) {
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

func (m model) sidebarRow(i int, view string, width int) string {
	count := len(todo.Filter(m.store.Tasks, view, "", time.Now()))
	label := fmt.Sprintf("%s %d", view, count)
	row := truncate(label, width)
	if i == m.sidebar {
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
	return fmt.Sprintf("%s %s", titleStyle.Render(fmt.Sprintf("%d tasks", len(tasks))), mutedStyle.Render(fmt.Sprintf("(%d open total)", open)))
}

func (m model) taskRow(index int, task todo.Task, width int) string {
	check := " "
	if task.Completed {
		check = "x"
	}
	cursor := "  "
	if index == m.selected {
		cursor = "> "
	}
	due := dueBadge(task)
	priority := priorityBadge(task.Priority)
	project := mutedStyle.Render("#" + task.Project)
	labels := ""
	if len(task.Labels) > 0 {
		parts := make([]string, len(task.Labels))
		for i, label := range task.Labels {
			parts[i] = "@" + label
		}
		labels = mutedStyle.Render(" " + strings.Join(parts, " "))
	}
	textBudget := width - ansi.StringWidth(cursor) - 4 - ansi.StringWidth(priority) - ansi.StringWidth(due) - ansi.StringWidth(project) - ansi.StringWidth(labels)
	if textBudget < 10 {
		textBudget = 10
	}
	title := truncate(task.Title, textBudget)
	row := fmt.Sprintf("%s[%s] %s %s %s %s%s", cursor, check, priority, title, due, project, labels)
	if index == m.selected {
		return selectedStyle.Render(pad(row, width))
	}
	return row
}

func dueBadge(task todo.Task) string {
	if task.Due == "" {
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
		return warnStyle.Render(task.Due)
	case d.Equal(start):
		return accentStyle.Render("today")
	case d.Equal(start.AddDate(0, 0, 1)):
		return "tomorrow"
	default:
		return task.Due
	}
}

func priorityBadge(priority int) string {
	switch priority {
	case 1:
		return warnStyle.Render("p1")
	case 2:
		return accentStyle.Render("p2")
	case 3:
		return "p3"
	default:
		return mutedStyle.Render("p4")
	}
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
