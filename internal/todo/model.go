package todo

import (
	"sort"
	"strings"
	"time"
)

type Task struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Project     string     `json:"project"`
	Due         string     `json:"due,omitempty"`
	Priority    int        `json:"priority"`
	Labels      []string   `json:"labels,omitempty"`
	Completed   bool       `json:"completed"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Store struct {
	NextID int    `json:"next_id"`
	Tasks  []Task `json:"tasks"`
}

type ParsedText struct {
	Title       string
	Project     string
	Due         string
	Priority    int
	Labels      []string
	HasProject  bool
	HasDue      bool
	HasPriority bool
	HasLabels   bool
}

func NewStore() Store {
	return Store{NextID: 1}
}

func (s *Store) Add(title, project string) int {
	now := time.Now()
	parsed := ParseTaskText(title, now)
	if parsed.Title == "" {
		return 0
	}
	project = CleanProject(project)
	if project == "" {
		project = "Inbox"
	}
	if parsed.HasProject {
		project = parsed.Project
	}
	priority := 4
	if parsed.HasPriority {
		priority = parsed.Priority
	}
	id := s.NextID
	s.NextID++
	s.Tasks = append(s.Tasks, Task{
		ID:        id,
		Title:     parsed.Title,
		Project:   project,
		Due:       parsed.Due,
		Priority:  priority,
		Labels:    parsed.Labels,
		CreatedAt: now,
	})
	return id
}

func (s *Store) Delete(id int) bool {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			s.Tasks = append(s.Tasks[:i], s.Tasks[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) Task(id int) (*Task, bool) {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			return &s.Tasks[i], true
		}
	}
	return nil, false
}

func (t *Task) ToggleComplete() {
	t.Completed = !t.Completed
	if t.Completed {
		now := time.Now()
		t.CompletedAt = &now
		return
	}
	t.CompletedAt = nil
}

func (t Task) DueTime() (time.Time, bool) {
	return t.DueTimeAt(time.Now())
}

func (t Task) DueTimeAt(now time.Time) (time.Time, bool) {
	if strings.TrimSpace(t.Due) == "" {
		return time.Time{}, false
	}
	d, err := ParseDue(t.Due, now)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

func ParseDue(input string, now time.Time) (time.Time, error) {
	input = strings.TrimSpace(strings.ToLower(input))
	today := startOfDay(now)
	switch input {
	case "", "none", "clear":
		return time.Time{}, nil
	case "today":
		return today, nil
	case "tomorrow", "tmr":
		return today.AddDate(0, 0, 1), nil
	}
	if strings.HasPrefix(input, "+") && strings.HasSuffix(input, "d") {
		n, ok := parsePositiveInt(strings.TrimSuffix(strings.TrimPrefix(input, "+"), "d"))
		if ok {
			return today.AddDate(0, 0, n), nil
		}
	}
	return time.ParseInLocation("2006-01-02", input, now.Location())
}

func NormalizeDue(input string, now time.Time) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" || strings.EqualFold(input, "none") || strings.EqualFold(input, "clear") {
		return "", nil
	}
	d, err := ParseDue(input, now)
	if err != nil {
		return "", err
	}
	return d.Format("2006-01-02"), nil
}

func ParseTaskText(input string, now time.Time) ParsedText {
	fields := strings.Fields(strings.TrimSpace(input))
	parsed := ParsedText{}
	var title []string
	for _, field := range fields {
		switch {
		case isPriority(field):
			parsed.Priority = int(field[1] - '0')
			parsed.HasPriority = true
		case strings.HasPrefix(field, "#") && CleanProject(field) != "":
			parsed.Project = CleanProject(field)
			parsed.HasProject = true
		case strings.HasPrefix(field, "@") && strings.Trim(strings.TrimSpace(field), "@") != "":
			parsed.Labels = append(parsed.Labels, strings.Trim(strings.TrimSpace(field), "@"))
			parsed.HasLabels = true
		case isDueToken(field, now):
			due, _ := NormalizeDue(field, now)
			parsed.Due = due
			parsed.HasDue = true
		default:
			title = append(title, field)
		}
	}
	parsed.Title = strings.Join(title, " ")
	if parsed.HasLabels {
		parsed.Labels = cleanLabelList(parsed.Labels)
	}
	return parsed
}

func ApplyTaskText(task *Task, input string, now time.Time) bool {
	parsed := ParseTaskText(input, now)
	if parsed.Title == "" {
		return false
	}
	task.Title = parsed.Title
	if parsed.HasProject {
		task.Project = parsed.Project
	}
	if parsed.HasDue {
		task.Due = parsed.Due
	}
	if parsed.HasPriority {
		task.Priority = parsed.Priority
	}
	if parsed.HasLabels {
		task.Labels = parsed.Labels
	}
	return true
}

func CleanProject(project string) string {
	project = strings.TrimSpace(project)
	project = strings.TrimPrefix(project, "#")
	return strings.TrimSpace(project)
}

func CleanLabels(input string) []string {
	var labels []string
	for _, field := range strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ' '
	}) {
		label := strings.Trim(strings.TrimSpace(field), "@")
		labels = append(labels, label)
	}
	return cleanLabelList(labels)
}

func cleanLabelList(input []string) []string {
	seen := map[string]bool{}
	var labels []string
	for _, label := range input {
		label = strings.Trim(strings.TrimSpace(label), "@")
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

func Projects(tasks []Task) []string {
	seen := map[string]bool{"Inbox": true}
	projects := []string{"Inbox"}
	for _, task := range tasks {
		project := CleanProject(task.Project)
		if project == "" || seen[project] {
			continue
		}
		seen[project] = true
		projects = append(projects, project)
	}
	sort.Strings(projects[1:])
	return projects
}

func Labels(tasks []Task) []string {
	seen := map[string]bool{}
	var labels []string
	for _, task := range tasks {
		for _, label := range task.Labels {
			if label == "" || seen[label] {
				continue
			}
			seen[label] = true
			labels = append(labels, label)
		}
	}
	sort.Strings(labels)
	return labels
}

func Filter(tasks []Task, view string, query string, now time.Time) []Task {
	query = strings.ToLower(strings.TrimSpace(query))
	today := startOfDay(now)
	var out []Task
	for _, task := range tasks {
		if !matchesView(task, view, today, now) {
			continue
		}
		if query != "" && !matchesQuery(task, query) {
			continue
		}
		out = append(out, task)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		da, oka := a.DueTimeAt(now)
		db, okb := b.DueTimeAt(now)
		if oka != okb {
			return oka
		}
		if oka && !da.Equal(db) {
			return da.Before(db)
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.ID < b.ID
	})
	return out
}

func matchesView(task Task, view string, today time.Time, now time.Time) bool {
	switch {
	case view == "Inbox":
		return !task.Completed && task.Project == "Inbox"
	case view == "Today":
		d, ok := task.DueTimeAt(now)
		return !task.Completed && ok && !d.After(today)
	case view == "Upcoming":
		d, ok := task.DueTimeAt(now)
		return !task.Completed && ok && d.After(today)
	case view == "All":
		return !task.Completed
	case view == "Completed":
		return task.Completed
	case strings.HasPrefix(view, "#"):
		return !task.Completed && task.Project == strings.TrimPrefix(view, "#")
	case strings.HasPrefix(view, "@"):
		label := strings.TrimPrefix(view, "@")
		return !task.Completed && hasLabel(task, label)
	default:
		return !task.Completed
	}
}

func matchesQuery(task Task, query string) bool {
	if strings.Contains(strings.ToLower(task.Title), query) ||
		strings.Contains(strings.ToLower(task.Project), query) ||
		strings.Contains(strings.ToLower(task.Due), query) {
		return true
	}
	for _, label := range task.Labels {
		if strings.Contains(strings.ToLower(label), query) {
			return true
		}
	}
	return false
}

func hasLabel(task Task, label string) bool {
	for _, candidate := range task.Labels {
		if candidate == label {
			return true
		}
	}
	return false
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func parsePositiveInt(input string) (int, bool) {
	if input == "" {
		return 0, false
	}
	n := 0
	for _, r := range input {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func isPriority(input string) bool {
	input = strings.ToLower(strings.TrimSpace(input))
	return len(input) == 2 && input[0] == 'p' && input[1] >= '1' && input[1] <= '4'
}

func isDueToken(input string, now time.Time) bool {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "today" || input == "tomorrow" || input == "tmr" {
		return true
	}
	if strings.HasPrefix(input, "+") && strings.HasSuffix(input, "d") {
		_, ok := parsePositiveInt(strings.TrimSuffix(strings.TrimPrefix(input, "+"), "d"))
		return ok
	}
	_, err := time.ParseInLocation("2006-01-02", input, now.Location())
	return err == nil
}
