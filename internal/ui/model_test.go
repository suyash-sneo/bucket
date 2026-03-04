package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bucket/internal/domain"
	uitheme "bucket/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeService struct {
	listCalls       int
	quickAddCalls   int
	updateCalls     int
	cycleCalls      int
	lastUpdatedTask domain.Task
	quickAddTask    domain.Task
	updateErr       error
	cycleErr        error
	listItems       []domain.TaskListItem
	getTask         domain.Task
	getSubtasks     []domain.Subtask
}

func (service *fakeService) QuickAdd(title string) (domain.Task, error) {
	service.quickAddCalls++
	task := service.quickAddTask
	task.Title = title
	if task.ID == 0 {
		task.ID = 1
	}
	if task.Status == "" {
		task.Status = domain.StatusCreated
	}
	if task.CreatedAt.IsZero() {
		now := time.Now().UTC()
		task.CreatedAt = now
		task.UpdatedAt = now
	}
	return task, nil
}

func (service *fakeService) CycleTaskStatus(id int64, baseUpdatedAt time.Time) (domain.Task, error) {
	service.cycleCalls++
	if service.cycleErr != nil {
		return domain.Task{}, service.cycleErr
	}
	return domain.Task{ID: id, Status: domain.StatusCompleted, UpdatedAt: time.Now().UTC()}, nil
}

func (service *fakeService) UpdateTask(baseUpdatedAt time.Time, task domain.Task) (domain.Task, error) {
	service.updateCalls++
	service.lastUpdatedTask = task
	if service.updateErr != nil {
		return domain.Task{}, service.updateErr
	}
	task.UpdatedAt = time.Now().UTC()
	return task, nil
}

func (service *fakeService) GetDetails(id int64) (domain.Task, []domain.Subtask, error) {
	if service.getTask.ID == 0 {
		service.getTask = domain.Task{ID: id, Title: "task", Status: domain.StatusCreated, UpdatedAt: time.Now().UTC(), CreatedAt: time.Now().UTC(), Meta: map[string]any{}}
	}
	return service.getTask, service.getSubtasks, nil
}

func (service *fakeService) List(listType string, now time.Time) ([]domain.TaskListItem, error) {
	service.listCalls++
	if service.listItems != nil {
		return service.listItems, nil
	}
	return []domain.TaskListItem{{ID: 1, Title: "task", Status: domain.StatusCreated, UpdatedAt: time.Now().UTC()}}, nil
}

func (service *fakeService) CreateSubtask(taskID int64, title string, position int) (domain.Subtask, error) {
	return domain.Subtask{ID: 1, TaskID: taskID, Title: title, Position: position, Status: domain.StatusCreated, UpdatedAt: time.Now().UTC()}, nil
}

func (service *fakeService) UpdateSubtask(baseUpdatedAt time.Time, subtask domain.Subtask) (domain.Subtask, error) {
	subtask.UpdatedAt = time.Now().UTC()
	return subtask, nil
}

func (service *fakeService) DeleteSubtask(id int64) error                   { return nil }
func (service *fakeService) ReorderSubtask(id int64, newPosition int) error { return nil }

func newTestModel(t *testing.T, svc *fakeService) *Model {
	t.Helper()
	if svc == nil {
		svc = &fakeService{}
	}
	m := NewModel(ModelOptions{
		Service:   svc,
		Theme:     uitheme.Dark(),
		ListType:  domain.ListInbox,
		DraftsDir: t.TempDir(),
		Now:       time.Now,
	})
	m.windowWidth = 120
	m.windowHeight = 40
	m.tasks = []domain.TaskListItem{{ID: 1, Title: "task", Status: domain.StatusCreated, UpdatedAt: time.Now().UTC()}}
	m.selectedIndex = 0
	m.selectedTaskID = 1
	m.details = domain.Task{ID: 1, Title: "task", Status: domain.StatusCreated, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Meta: map[string]any{}}
	m.syncFormWithTask(m.details)
	return m
}

func keyRunes(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func keyEnter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }
func keyEsc() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyEsc} }
func keySpace() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}} }
func keyTab() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyTab} }
func keyCtrlQ() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyCtrlQ} }
func keyCtrlN() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyCtrlN} }

func TestKeyAEntersQuickAddMode(t *testing.T) {
	m := newTestModel(t, &fakeService{})
	_, _ = m.Update(keyRunes('a'))
	if m.mode != ModeQuickAdd {
		t.Fatalf("expected quick add mode, got %v", m.mode)
	}
}

func TestQuickAddEnterCreatesTaskAndEntersEdit(t *testing.T) {
	svc := &fakeService{quickAddTask: domain.Task{ID: 2, Status: domain.StatusCreated, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Meta: map[string]any{}}}
	m := newTestModel(t, svc)
	_, _ = m.Update(keyRunes('a'))
	m.quickAddInput.SetValue("new task")
	_, cmd := m.Update(keyEnter())
	if cmd == nil {
		t.Fatalf("expected quick add command")
	}
	msg := cmd()
	_, _ = m.Update(msg)
	if svc.quickAddCalls != 1 {
		t.Fatalf("expected quick add call")
	}
	if m.mode != ModeEdit {
		t.Fatalf("expected edit mode after quick add, got %v", m.mode)
	}
	if m.editDraft.ID != 2 {
		t.Fatalf("expected selected draft task id=2, got %d", m.editDraft.ID)
	}
}

func TestInitialListLoadFetchesDetailsForSelectedTask(t *testing.T) {
	now := time.Now().UTC()
	svc := &fakeService{
		listItems: []domain.TaskListItem{
			{ID: 5, Title: "first", Status: domain.StatusCreated, UpdatedAt: now},
		},
		getTask: domain.Task{
			ID:        5,
			Title:     "first",
			Status:    domain.StatusCreated,
			CreatedAt: now,
			UpdatedAt: now,
			Meta:      map[string]any{},
		},
	}
	m := NewModel(ModelOptions{
		Service:   svc,
		Theme:     uitheme.Dark(),
		ListType:  domain.ListInbox,
		DraftsDir: t.TempDir(),
		Now:       time.Now,
	})

	initCmd := m.Init()
	if initCmd == nil {
		t.Fatalf("expected init list load command")
	}
	_, detailsCmd := m.Update(initCmd())
	if detailsCmd == nil {
		t.Fatalf("expected details load command after initial list load")
	}
	_, _ = m.Update(detailsCmd())
	if m.selectedTaskID != 5 {
		t.Fatalf("expected selected task ID 5, got %d", m.selectedTaskID)
	}
	if m.details.ID != 5 {
		t.Fatalf("expected details for selected task, got ID %d", m.details.ID)
	}
	if m.detailsLoadedAt.IsZero() {
		t.Fatalf("expected detailsLoadedAt to be set")
	}
}

func TestSpaceCyclesStatusInListMode(t *testing.T) {
	svc := &fakeService{}
	m := newTestModel(t, svc)
	_, cmd := m.Update(keySpace())
	if cmd == nil {
		t.Fatalf("expected status cycle command")
	}
	if m.tasks[0].Status != domain.StatusInProgress {
		t.Fatalf("expected optimistic status change, got %s", m.tasks[0].Status)
	}
	msg := cmd()
	_, _ = m.Update(msg)
	if m.tasks[0].Status != domain.StatusCompleted {
		t.Fatalf("expected persisted status, got %s", m.tasks[0].Status)
	}
}

func TestEnterAndEscEditTransitions(t *testing.T) {
	m := newTestModel(t, &fakeService{})
	_, _ = m.Update(keyEnter())
	if m.mode != ModeEdit {
		t.Fatalf("expected edit mode, got %v", m.mode)
	}
	_, _ = m.Update(keyEsc())
	if m.mode != ModeList {
		t.Fatalf("expected list mode, got %v", m.mode)
	}
}

func TestCtrlQTriggersQuit(t *testing.T) {
	m := newTestModel(t, &fakeService{})
	_, cmd := m.Update(keyCtrlQ())
	if cmd == nil {
		t.Fatalf("expected quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg")
	}
}

func TestTabCyclesFieldOrder(t *testing.T) {
	m := newTestModel(t, &fakeService{})
	m.mode = ModeEdit
	m.form.focused = FieldTitle
	_, _ = m.Update(keyTab())
	if m.form.focused != FieldStatus {
		t.Fatalf("expected focus FieldStatus, got %v", m.form.focused)
	}
	_, _ = m.Update(keyTab())
	if m.form.focused != FieldURL {
		t.Fatalf("expected focus FieldURL, got %v", m.form.focused)
	}
}

func TestEditTypingDoesNotTriggerFocusHotkeys(t *testing.T) {
	m := newTestModel(t, &fakeService{})
	m.mode = ModeEdit
	m.form.focused = FieldTitle
	m.focusField()
	m.form.title.SetValue("")

	_, _ = m.Update(keyRunes('n'))
	if m.mode != ModeEdit {
		t.Fatalf("expected to remain in edit mode while typing, got %v", m.mode)
	}
	if m.form.focused != FieldTitle {
		t.Fatalf("expected focus to stay on title while typing, got %v", m.form.focused)
	}
	if m.form.title.Value() != "n" {
		t.Fatalf("expected typed rune in title input, got %q", m.form.title.Value())
	}
}

func TestEditCtrlNStillEntersNotesModeFromTextField(t *testing.T) {
	m := newTestModel(t, &fakeService{})
	m.mode = ModeEdit
	m.form.focused = FieldTitle
	m.focusField()

	_, _ = m.Update(keyCtrlN())
	if m.mode != ModeNotesEdit {
		t.Fatalf("expected ctrl+n to open notes mode, got %v", m.mode)
	}
}

func TestFilterModeEscClearsAndReloads(t *testing.T) {
	svc := &fakeService{}
	m := newTestModel(t, svc)
	_, _ = m.Update(keyRunes('/'))
	if m.mode != ModeFilterInput {
		t.Fatalf("expected filter mode")
	}
	m.filterInput.SetValue("abc")
	m.filterQuery = "abc"
	m.filterActive = true
	_, cmd := m.Update(keyEsc())
	if m.mode != ModeList {
		t.Fatalf("expected list mode after esc")
	}
	if m.filterQuery != "" || m.filterActive {
		t.Fatalf("expected filter cleared")
	}
	if cmd == nil {
		t.Fatalf("expected reload command")
	}
	_, _ = m.Update(cmd())
	if svc.listCalls == 0 {
		t.Fatalf("expected list reload call")
	}
}

func TestNotesModeTransitions(t *testing.T) {
	m := newTestModel(t, &fakeService{})
	m.mode = ModeEdit
	m.form.focused = FieldStatus
	_, _ = m.Update(keyCtrlN())
	if m.mode != ModeNotesEdit {
		t.Fatalf("expected notes edit mode, got %v", m.mode)
	}
	_, _ = m.Update(keyEsc())
	if m.mode != ModeEdit {
		t.Fatalf("expected edit mode after esc, got %v", m.mode)
	}
}

func TestAutosaveTickSchedulesAndRetries(t *testing.T) {
	svc := &fakeService{updateErr: errors.New("boom")}
	m := newTestModel(t, svc)
	m.mode = ModeEdit
	m.dirty = true

	_, cmd := m.Update(autosaveTickMsg{time: time.Now()})
	if !m.saveInFlight {
		t.Fatalf("expected save to be in flight")
	}
	if cmd == nil {
		t.Fatalf("expected autosave command")
	}
	_, _ = m.Update(saveResultMsg{requestID: m.pending.saveID, err: errors.New("boom")})
	if !m.dirty {
		t.Fatalf("expected dirty to remain true after failure")
	}
	if m.saveInFlight {
		t.Fatalf("expected saveInFlight false after failure result")
	}

	_, _ = m.Update(autosaveTickMsg{time: time.Now().Add(time.Second)})
	if !m.saveInFlight {
		t.Fatalf("expected retry save to be scheduled")
	}
}

func TestQuitDirtyWritesDraftBeforeExit(t *testing.T) {
	draftsDir := t.TempDir()
	svc := &fakeService{updateErr: errors.New("save failed")}
	m := NewModel(ModelOptions{
		Service:   svc,
		Theme:     uitheme.Dark(),
		ListType:  domain.ListInbox,
		DraftsDir: draftsDir,
		Now:       time.Now,
	})
	m.windowWidth = 100
	m.windowHeight = 30
	m.mode = ModeEdit
	m.editDraft = domain.Task{ID: 42, Title: "dirty", Status: domain.StatusCreated, UpdatedAt: time.Now().UTC(), CreatedAt: time.Now().UTC(), Meta: map[string]any{}}
	m.baseUpdatedAt = m.editDraft.UpdatedAt
	m.syncFormWithTask(m.editDraft)
	m.form.title.SetValue("dirty")
	m.dirty = true

	_, cmd := m.Update(keyCtrlQ())
	if cmd == nil {
		t.Fatalf("expected quit command")
	}
	path := filepath.Join(draftsDir, "task-42.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected draft file to be written before quit: %v", err)
	}
}

func TestQuickAddTypingUppercaseIDoesNotSwitchList(t *testing.T) {
	m := newTestModel(t, &fakeService{})
	m.listType = domain.ListUpcoming

	_, _ = m.Update(keyRunes('a'))
	if m.mode != ModeQuickAdd {
		t.Fatalf("expected quick add mode, got %v", m.mode)
	}

	_, _ = m.Update(keyRunes('I'))
	if m.mode != ModeQuickAdd {
		t.Fatalf("expected to stay in quick add mode while typing, got %v", m.mode)
	}
	if m.listType != domain.ListUpcoming {
		t.Fatalf("expected list type unchanged while typing, got %s", m.listType)
	}
	if m.quickAddInput.Value() != "I" {
		t.Fatalf("expected input to capture typed rune, got %q", m.quickAddInput.Value())
	}
}

func TestEmptyWideViewShowsDemarcatedPanes(t *testing.T) {
	m := NewModel(ModelOptions{
		Service:   &fakeService{listItems: []domain.TaskListItem{}},
		Theme:     uitheme.Dark(),
		ListType:  domain.ListInbox,
		DraftsDir: t.TempDir(),
		Now:       time.Now,
	})
	m.windowWidth = 120
	m.windowHeight = 40
	m.tasks = nil
	m.selectedIndex = 0
	m.mode = ModeList

	view := m.View()
	if !strings.Contains(view, " Tasks") {
		t.Fatalf("expected tasks pane title in view")
	}
	if !strings.Contains(view, " Details") {
		t.Fatalf("expected details pane title in view")
	}
	if !strings.Contains(view, "No task selected") {
		t.Fatalf("expected empty details placeholder")
	}
	if strings.Contains(view, "URL: (none)") {
		t.Fatalf("did not expect empty task fields when no task is selected")
	}
}
