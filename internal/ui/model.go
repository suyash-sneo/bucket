package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"bucket/internal/domain"
	"bucket/internal/service"
	"bucket/internal/ui/components"
	uitheme "bucket/internal/ui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	Version = "dev"
)

type Service interface {
	QuickAdd(title string) (domain.Task, error)
	CycleTaskStatus(id int64, baseUpdatedAt time.Time) (domain.Task, error)
	UpdateTask(baseUpdatedAt time.Time, task domain.Task) (domain.Task, error)
	GetDetails(id int64) (domain.Task, []domain.Subtask, error)
	List(listType string, now time.Time) ([]domain.TaskListItem, error)
	CreateSubtask(taskID int64, title string, position int) (domain.Subtask, error)
	UpdateSubtask(baseUpdatedAt time.Time, subtask domain.Subtask) (domain.Subtask, error)
	DeleteSubtask(id int64) error
	ReorderSubtask(id int64, newPosition int) error
}

type Mode int

const (
	ModeList Mode = iota
	ModeFilterInput
	ModeQuickAdd
	ModeEdit
	ModeNotesEdit
	ModeSubtasks
	ModeModalError
	ModeConfirmDelete
)

type FieldFocus int

const (
	FieldTitle FieldFocus = iota
	FieldStatus
	FieldURL
	FieldDue
	FieldPriority
	FieldEstimate
	FieldProgress
	FieldSubtasks
	FieldNotes
)

var fieldOrder = []FieldFocus{
	FieldTitle,
	FieldStatus,
	FieldURL,
	FieldDue,
	FieldPriority,
	FieldEstimate,
	FieldProgress,
	FieldSubtasks,
	FieldNotes,
}

type ModelOptions struct {
	Service        Service
	Theme          uitheme.Theme
	ListType       string
	DraftsDir      string
	Editor         string
	ConflictDrafts []string
	Now            func() time.Time
	Logger         *slog.Logger
}

type Model struct {
	service Service
	mode    Mode

	listType string
	tasks    []domain.TaskListItem

	selectedIndex  int
	selectedTaskID int64

	filterQuery   string
	filterActive  bool
	filteredIdx   []int
	filterInput   textinput.Model
	quickAddInput textinput.Model

	details         domain.Task
	subtasks        []domain.Subtask
	detailsLoadedAt time.Time

	form        editorForm
	notesEditor textarea.Model
	viewport    viewport.Model

	subtaskIndex      int
	subtaskInput      textinput.Model
	subtaskInputMode  string
	subtaskEditBase   time.Time
	subtaskEditTarget int64
	pendingDeleteID   int64

	toastMessage string
	toastExpiry  time.Time

	help  help.Model
	keys  KeyMap
	theme uitheme.Theme

	errModal modalState

	pending requestState

	dirty        bool
	saveInFlight bool
	lastSaveErr  error

	baseUpdatedAt time.Time
	editDraft     domain.Task

	markdown *components.MarkdownRenderer

	draftsDir string
	editor    string
	logger    *slog.Logger
	debugKeys bool

	windowWidth  int
	windowHeight int

	selectAfterReload int64

	now func() time.Time
}

type editorForm struct {
	focused  FieldFocus
	title    textinput.Model
	url      textinput.Model
	due      textinput.Model
	priority textinput.Model
	estimate textinput.Model
	progress textinput.Model
}

type modalState struct {
	Title string
	Body  string
	Fatal bool
}

type requestState struct {
	nextID      int
	listID      int
	detailsID   int
	saveID      int
	statusID    int
	quickAddID  int
	subtaskOpID int
}

func NewModel(options ModelOptions) *Model {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.ListType == "" {
		options.ListType = domain.ListInbox
	}
	debugKeys := os.Getenv("BUCKET_DEBUG_KEYS") != ""
	quickAdd := textinput.New()
	quickAdd.Placeholder = "Task title"
	quickAdd.CharLimit = 256
	quickAdd.Prompt = ""
	applyCursorStyle(&quickAdd, options.Theme)

	filterInput := textinput.New()
	filterInput.Placeholder = "title fuzzy filter"
	filterInput.Prompt = ""
	applyCursorStyle(&filterInput, options.Theme)

	titleInput := textinput.New()
	titleInput.CharLimit = 256
	titleInput.Prompt = ""
	applyCursorStyle(&titleInput, options.Theme)

	urlInput := textinput.New()
	urlInput.CharLimit = 2048
	urlInput.Prompt = ""
	applyCursorStyle(&urlInput, options.Theme)

	dueInput := textinput.New()
	dueInput.Placeholder = "YYYY-MM-DD or YYYY-MM-DD HH:MM"
	dueInput.Prompt = ""
	applyCursorStyle(&dueInput, options.Theme)

	priorityInput := textinput.New()
	priorityInput.Placeholder = "1..4"
	priorityInput.Prompt = ""
	applyCursorStyle(&priorityInput, options.Theme)

	estimateInput := textinput.New()
	estimateInput.Placeholder = "90 or 1h30m"
	estimateInput.Prompt = ""
	applyCursorStyle(&estimateInput, options.Theme)

	progressInput := textinput.New()
	progressInput.Placeholder = "0..100"
	progressInput.Prompt = ""
	applyCursorStyle(&progressInput, options.Theme)

	subtaskInput := textinput.New()
	subtaskInput.Prompt = ""
	subtaskInput.CharLimit = 256
	applyCursorStyle(&subtaskInput, options.Theme)

	notes := textarea.New()
	notes.SetHeight(10)
	notes.SetWidth(10)
	notes.ShowLineNumbers = false
	notes.CharLimit = 0

	h := help.New()
	h.ShowAll = false

	model := &Model{
		service:       options.Service,
		mode:          ModeList,
		listType:      options.ListType,
		filterInput:   filterInput,
		quickAddInput: quickAdd,
		form: editorForm{
			focused:  FieldTitle,
			title:    titleInput,
			url:      urlInput,
			due:      dueInput,
			priority: priorityInput,
			estimate: estimateInput,
			progress: progressInput,
		},
		notesEditor:  notes,
		subtaskInput: subtaskInput,
		viewport:     viewport.New(0, 0),
		help:         h,
		keys:         DefaultKeyMap(),
		theme:        options.Theme,
		markdown:     components.NewMarkdownRenderer(),
		draftsDir:    options.DraftsDir,
		editor:       options.Editor,
		now:          options.Now,
		logger:       options.Logger,
		debugKeys:    debugKeys,
	}
	if len(options.ConflictDrafts) > 0 {
		model.mode = ModeModalError
		model.errModal = modalState{
			Title: "Draft conflicts detected",
			Body:  strings.Join(options.ConflictDrafts, "\n"),
			Fatal: false,
		}
	}
	return model
}

func applyCursorStyle(input *textinput.Model, palette uitheme.Theme) {
	input.Cursor.Style = lipgloss.NewStyle().
		Foreground(palette.BG).
		Background(palette.Accent).
		Bold(true)
}

func (model *Model) Init() tea.Cmd {
	if model.mode == ModeModalError {
		return tea.Batch(model.reloadListCmd(), model.autosaveTickCmd())
	}
	return model.reloadListCmd()
}

type listLoadedMsg struct {
	requestID int
	items     []domain.TaskListItem
	err       error
}

type detailsLoadedMsg struct {
	requestID int
	task      domain.Task
	subtasks  []domain.Subtask
	err       error
}

type quickAddDoneMsg struct {
	requestID int
	task      domain.Task
	err       error
}

type statusCycleMsg struct {
	requestID int
	taskID    int64
	updated   domain.Task
	err       error
}

type saveResultMsg struct {
	requestID int
	task      domain.Task
	err       error
}

type subtaskResultMsg struct {
	requestID int
	err       error
}

type autosaveTickMsg struct {
	time time.Time
}

type toastClearMsg struct {
	time time.Time
}

type urlOpenResultMsg struct {
	err error
}

type notesExternalEditMsg struct {
	notes string
	err   error
}

func (model *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := message.(type) {
	case tea.WindowSizeMsg:
		model.windowWidth = typed.Width
		model.windowHeight = typed.Height
		model.viewport.Width = maxInt(1, model.rightPaneWidth())
		model.viewport.Height = maxInt(1, model.contentHeight())
		model.notesEditor.SetWidth(maxInt(20, model.rightPaneWidth()-1))
		model.notesEditor.SetHeight(maxInt(5, model.contentHeight()))
		return model, nil
	case listLoadedMsg:
		if typed.requestID != model.pending.listID {
			return model, nil
		}
		if typed.err != nil {
			return model, model.setToast("Failed to load tasks")
		}
		model.tasks = typed.items
		if model.filterActive {
			model.applyCurrentFilter()
		}
		if model.selectAfterReload != 0 {
			model.selectTaskByID(model.selectAfterReload)
			model.selectAfterReload = 0
		}
		model.clampSelection()
		if task, ok := model.selectedTask(); ok {
			model.selectedTaskID = task.ID
			if model.details.ID != task.ID || model.detailsLoadedAt.IsZero() {
				return model, model.loadDetailsCmd(task.ID)
			}
			return model, nil
		}
		model.details = domain.Task{}
		model.subtasks = nil
		model.detailsLoadedAt = time.Time{}
		model.viewport.SetContent("")
		return model, nil
	case detailsLoadedMsg:
		if typed.requestID != model.pending.detailsID {
			return model, nil
		}
		if typed.err != nil {
			return model, model.setToast("Failed to load details")
		}
		model.details = typed.task
		model.subtasks = typed.subtasks
		model.detailsLoadedAt = model.now()
		if model.mode == ModeEdit || model.mode == ModeNotesEdit || model.mode == ModeSubtasks {
			model.syncFormWithTask(typed.task)
		}
		notes, err := model.markdown.Render(typed.task.ID, typed.task.Notes, model.theme.Name)
		if err != nil {
			model.viewport.SetContent(typed.task.Notes)
		} else {
			model.viewport.SetContent(notes)
		}
		return model, nil
	case quickAddDoneMsg:
		if typed.requestID != model.pending.quickAddID {
			return model, nil
		}
		if typed.err != nil {
			model.mode = ModeList
			return model, model.setToast("Failed to create task")
		}
		model.selectAfterReload = typed.task.ID
		model.selectedTaskID = typed.task.ID
		model.mode = ModeEdit
		model.editDraft = typed.task
		model.baseUpdatedAt = typed.task.UpdatedAt
		model.syncFormWithTask(typed.task)
		model.form.focused = FieldTitle
		model.focusField()
		model.dirty = false
		model.lastSaveErr = nil
		return model, tea.Batch(model.reloadListCmd(), model.loadDetailsCmd(typed.task.ID), model.autosaveTickCmd())
	case statusCycleMsg:
		if typed.requestID != model.pending.statusID {
			return model, nil
		}
		if typed.err != nil {
			if index := model.taskIndexByID(typed.taskID); index >= 0 {
				model.tasks[index].Status = domain.CycleStatus(domain.CycleStatus(domain.CycleStatus(domain.CycleStatus(model.tasks[index].Status))))
			}
			return model, model.setToast("Status update failed")
		}
		if index := model.taskIndexByID(typed.updated.ID); index >= 0 {
			model.tasks[index].Status = typed.updated.Status
			model.tasks[index].UpdatedAt = typed.updated.UpdatedAt
		}
		if model.selectedTaskID == typed.updated.ID {
			model.details.Status = typed.updated.Status
			model.editDraft.Status = typed.updated.Status
			model.baseUpdatedAt = typed.updated.UpdatedAt
		}
		return model, nil
	case saveResultMsg:
		if typed.requestID != model.pending.saveID {
			return model, nil
		}
		model.saveInFlight = false
		if typed.err != nil {
			model.lastSaveErr = typed.err
			model.dirty = true
			_, _ = model.writeDraftNow()
			if service.IsConflict(typed.err) {
				path := DraftFilePath(model.draftsDir, model.editDraft.ID)
				model.mode = ModeModalError
				model.errModal = modalState{
					Title: "Conflict detected",
					Body:  "Your edits were saved to draft:\n" + path,
					Fatal: false,
				}
			}
			return model, model.setToast("Autosave failed")
		}
		model.lastSaveErr = nil
		model.dirty = false
		model.editDraft = typed.task
		model.baseUpdatedAt = typed.task.UpdatedAt
		model.details = typed.task
		_ = DeleteDraftFile(model.draftsDir, typed.task.ID)
		return model, model.reloadListCmd()
	case subtaskResultMsg:
		if typed.requestID != model.pending.subtaskOpID {
			return model, nil
		}
		if typed.err != nil {
			return model, model.setToast("Subtask action failed")
		}
		if model.editDraft.ID != 0 {
			return model, model.loadDetailsCmd(model.editDraft.ID)
		}
		return model, nil
	case autosaveTickMsg:
		if model.mode == ModeEdit || model.mode == ModeNotesEdit {
			cmd := model.autosaveTickCmd()
			if model.dirty && !model.saveInFlight {
				if _, err := model.writeDraftNow(); err != nil {
					return model, tea.Batch(cmd, model.setToast("Autosave failed"))
				}
				saveCmd := model.startSaveCmd()
				if saveCmd != nil {
					return model, tea.Batch(cmd, saveCmd)
				}
			}
			return model, cmd
		}
		return model, nil
	case toastClearMsg:
		if typed.time.Equal(model.toastExpiry) {
			model.toastMessage = ""
		}
		return model, nil
	case urlOpenResultMsg:
		if typed.err != nil {
			return model, model.setToast(typed.err.Error())
		}
		return model, nil
	case notesExternalEditMsg:
		if typed.err != nil {
			return model, model.setToast("External editor failed")
		}
		model.notesEditor.SetValue(typed.notes)
		model.editDraft.Notes = typed.notes
		model.dirty = true
		return model, nil
	case tea.KeyMsg:
		model.logKey("received", typed)
		if model.mode == ModeModalError {
			// Allow dismiss with Esc (default cancel) or Enter so users are never stuck
			if key.Matches(typed, model.keys.Cancel) || typed.Type == tea.KeyEnter {
				if model.errModal.Fatal {
					return model, tea.Quit
				}
				model.mode = ModeList
				model.errModal = modalState{}
			}
			return model, nil
		}

		if key.Matches(typed, model.keys.Quit, model.keys.QuitSIG) {
			return model, model.handleQuit()
		}

		if model.shouldHandleGlobalListSwitch() {
			if cmd := model.handleGlobalListSwitch(typed); cmd != nil {
				return model, cmd
			}
		}

		switch model.mode {
		case ModeList:
			return model, model.updateListMode(typed)
		case ModeFilterInput:
			return model, model.updateFilterMode(typed)
		case ModeQuickAdd:
			return model, model.updateQuickAddMode(typed)
		case ModeEdit:
			return model, model.updateEditMode(typed)
		case ModeNotesEdit:
			return model, model.updateNotesMode(typed)
		case ModeSubtasks:
			return model, model.updateSubtasksMode(typed)
		case ModeConfirmDelete:
			return model, model.updateConfirmDeleteMode(typed)
		}
	}
	return model, nil
}

func (model *Model) logKey(stage string, message tea.KeyMsg) {
	if !model.debugKeys || model.logger == nil {
		return
	}
	model.logger.Info("key_debug",
		"stage", stage,
		"mode", modeString(model.mode),
		"focused_field", model.focusedFieldName(),
		"key_string", message.String(),
		"key_type", int(message.Type),
		"key_runes", fmt.Sprintf("%+q", message.Runes),
		"alt", message.Alt,
		"paste", message.Paste,
	)
}

func modeString(mode Mode) string {
	switch mode {
	case ModeList:
		return "list"
	case ModeFilterInput:
		return "filter"
	case ModeQuickAdd:
		return "quickadd"
	case ModeEdit:
		return "edit"
	case ModeNotesEdit:
		return "notes"
	case ModeSubtasks:
		return "subtasks"
	case ModeModalError:
		return "modal_error"
	case ModeConfirmDelete:
		return "confirm_delete"
	default:
		return "unknown"
	}
}

func (model *Model) shouldHandleGlobalListSwitch() bool {
	switch model.mode {
	case ModeQuickAdd, ModeFilterInput, ModeNotesEdit:
		return false
	case ModeEdit:
		switch model.form.focused {
		case FieldTitle, FieldURL, FieldDue, FieldPriority, FieldEstimate, FieldProgress:
			return false
		default:
			return true
		}
	case ModeSubtasks:
		return model.subtaskInputMode == ""
	default:
		return true
	}
}

func (model *Model) handleGlobalListSwitch(message tea.KeyMsg) tea.Cmd {
	newList := ""
	switch {
	case key.Matches(message, model.keys.Inbox):
		newList = domain.ListInbox
	case key.Matches(message, model.keys.Upcoming):
		newList = domain.ListUpcoming
	case key.Matches(message, model.keys.All):
		newList = domain.ListAll
	case key.Matches(message, model.keys.Closed):
		newList = domain.ListClosed
	case key.Matches(message, model.keys.Archived):
		newList = domain.ListArchived
	}
	if newList == "" {
		return nil
	}
	model.listType = newList
	model.mode = ModeList
	model.selectedIndex = 0
	model.filterQuery = ""
	model.filterActive = false
	model.filteredIdx = nil
	model.filterInput.SetValue("")
	return model.reloadListCmd()
}

func (model *Model) updateListMode(message tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(message, model.keys.Up):
		model.moveSelection(-1)
		if task, ok := model.selectedTask(); ok {
			return model.loadDetailsCmd(task.ID)
		}
	case key.Matches(message, model.keys.Down):
		model.moveSelection(1)
		if task, ok := model.selectedTask(); ok {
			return model.loadDetailsCmd(task.ID)
		}
	case key.Matches(message, model.keys.PageUp):
		model.moveSelection(-10)
		if task, ok := model.selectedTask(); ok {
			return model.loadDetailsCmd(task.ID)
		}
	case key.Matches(message, model.keys.PageDown):
		model.moveSelection(10)
		if task, ok := model.selectedTask(); ok {
			return model.loadDetailsCmd(task.ID)
		}
	case key.Matches(message, model.keys.Top):
		model.selectedIndex = 0
		if task, ok := model.selectedTask(); ok {
			return model.loadDetailsCmd(task.ID)
		}
	case key.Matches(message, model.keys.Bottom):
		visible := model.visibleIndices()
		if len(visible) > 0 {
			model.selectedIndex = len(visible) - 1
			if task, ok := model.selectedTask(); ok {
				return model.loadDetailsCmd(task.ID)
			}
		}
	case key.Matches(message, model.keys.QuickAdd):
		model.mode = ModeQuickAdd
		model.quickAddInput.SetValue("")
		model.quickAddInput.Focus()
		return nil
	case key.Matches(message, model.keys.Filter):
		model.mode = ModeFilterInput
		model.filterInput.SetValue(model.filterQuery)
		model.filterInput.Focus()
		return nil
	case key.Matches(message, model.keys.Cycle):
		task, ok := model.selectedTask()
		if !ok {
			return nil
		}
		index := model.taskIndexByID(task.ID)
		if index >= 0 {
			model.tasks[index].Status = domain.CycleStatus(model.tasks[index].Status)
		}
		requestID := model.nextRequestID(&model.pending.statusID)
		return cycleTaskStatusCmd(model.service, requestID, task.ID, task.UpdatedAt)
	case key.Matches(message, model.keys.OpenURL):
		return model.openCurrentTaskURLCmd()
	case key.Matches(message, model.keys.EnterEdit):
		task, ok := model.selectedTask()
		if !ok {
			return nil
		}
		model.mode = ModeEdit
		model.form.focused = FieldTitle
		model.focusField()
		cmds := []tea.Cmd{model.autosaveTickCmd()}
		if model.details.ID != task.ID {
			cmds = append(cmds, model.loadDetailsCmd(task.ID))
		} else {
			model.syncFormWithTask(model.details)
		}
		return tea.Batch(cmds...)
	}
	return nil
}

func (model *Model) updateFilterMode(message tea.KeyMsg) tea.Cmd {
	if key.Matches(message, model.keys.Cancel) {
		model.mode = ModeList
		model.filterQuery = ""
		model.filterInput.SetValue("")
		model.filterActive = false
		model.filteredIdx = nil
		return model.reloadListCmd()
	}
	if key.Matches(message, model.keys.Apply) {
		model.filterQuery = strings.TrimSpace(model.filterInput.Value())
		model.applyCurrentFilter()
		model.mode = ModeList
		return nil
	}
	updatedInput, cmd := model.filterInput.Update(message)
	model.filterInput = updatedInput
	return cmd
}

func (model *Model) updateQuickAddMode(message tea.KeyMsg) tea.Cmd {
	if key.Matches(message, model.keys.Cancel) {
		model.mode = ModeList
		return nil
	}
	if key.Matches(message, model.keys.Apply) {
		title := strings.TrimSpace(model.quickAddInput.Value())
		if title == "" {
			return model.setToast("Title required")
		}
		requestID := model.nextRequestID(&model.pending.quickAddID)
		return quickAddTaskCmd(model.service, requestID, title)
	}
	updatedInput, cmd := model.quickAddInput.Update(message)
	model.quickAddInput = updatedInput
	return cmd
}

func (model *Model) updateEditMode(message tea.KeyMsg) tea.Cmd {
	if key.Matches(message, model.keys.ExitEdit) {
		// When editing inline text inputs, left arrow should move the caret
		// instead of exiting edit mode.
		if message.Type == tea.KeyLeft && model.isTextInputFieldFocused() {
			before := model.formValuesHash()
			cmd := model.updateFocusedInput(message)
			after := model.formValuesHash()
			if before != after {
				model.dirty = true
			}
			return cmd
		}
		_, _ = model.writeDraftNow()
		_ = model.flushDirty(300 * time.Millisecond)
		model.mode = ModeList
		model.blurFields()
		return nil
	}
	if model.form.focused == FieldURL && model.matchesClearURL(message) {
		model.form.url.SetValue("")
		model.editDraft.URL = ""
		model.dirty = true
		return model.startSaveCmd()
	}
	if model.handleFieldSelectors(message) {
		return nil
	}
	if key.Matches(message, model.keys.CycleEdit) {
		model.editDraft.Status = domain.CycleStatus(model.editDraft.Status)
		model.dirty = true
		cmd := model.startSaveCmd()
		if cmd != nil {
			return cmd
		}
		return nil
	}
	if key.Matches(message, model.keys.OpenURLEdit) {
		return model.openCurrentTaskURLCmd()
	}
	if key.Matches(message, model.keys.TabNext) {
		model.focusNextField()
		if cmd := model.startSaveCmd(); cmd != nil {
			return cmd
		}
		return nil
	}
	if key.Matches(message, model.keys.TabPrev) {
		model.focusPrevField()
		if cmd := model.startSaveCmd(); cmd != nil {
			return cmd
		}
		return nil
	}
	if key.Matches(message, model.keys.FocusNotes) {
		model.mode = ModeNotesEdit
		model.notesEditor.Focus()
		return model.autosaveTickCmd()
	}
	if key.Matches(message, model.keys.FocusSubtasks) {
		model.mode = ModeSubtasks
		model.subtaskInputMode = ""
		model.subtaskInput.SetValue("")
		return nil
	}
	if key.Matches(message, model.keys.FocusTitle) {
		model.form.focused = FieldTitle
		model.focusField()
		if cmd := model.startSaveCmd(); cmd != nil {
			return cmd
		}
		return nil
	}
	if key.Matches(message, model.keys.FocusStatus) {
		model.form.focused = FieldStatus
		model.focusField()
		if cmd := model.startSaveCmd(); cmd != nil {
			return cmd
		}
		return nil
	}
	if key.Matches(message, model.keys.FocusURL) {
		model.form.focused = FieldURL
		model.focusField()
		if cmd := model.startSaveCmd(); cmd != nil {
			return cmd
		}
		return nil
	}
	if key.Matches(message, model.keys.FocusDue) {
		model.form.focused = FieldDue
		model.focusField()
		if cmd := model.startSaveCmd(); cmd != nil {
			return cmd
		}
		return nil
	}
	if key.Matches(message, model.keys.FocusPriority) {
		model.form.focused = FieldPriority
		model.focusField()
		if cmd := model.startSaveCmd(); cmd != nil {
			return cmd
		}
		return nil
	}
	if key.Matches(message, model.keys.FocusEstimate) {
		model.form.focused = FieldEstimate
		model.focusField()
		if cmd := model.startSaveCmd(); cmd != nil {
			return cmd
		}
		return nil
	}
	if key.Matches(message, model.keys.FocusProgress) {
		model.form.focused = FieldProgress
		model.focusField()
		if cmd := model.startSaveCmd(); cmd != nil {
			return cmd
		}
		return nil
	}
	if model.form.focused == FieldStatus && key.Matches(message, model.keys.Cycle) {
		model.editDraft.Status = domain.CycleStatus(model.editDraft.Status)
		model.dirty = true
		return model.startSaveCmd()
	}

	before := model.formValuesHash()
	cmd := model.updateFocusedInput(message)
	after := model.formValuesHash()
	if before != after {
		model.dirty = true
	}
	return cmd
}

func (model *Model) updateNotesMode(message tea.KeyMsg) tea.Cmd {
	if key.Matches(message, model.keys.Cancel) {
		_, _ = model.writeDraftNow()
		_ = model.flushDirty(300 * time.Millisecond)
		model.mode = ModeEdit
		model.notesEditor.Blur()
		model.focusField()
		return model.autosaveTickCmd()
	}
	if key.Matches(message, model.keys.OpenEditor) {
		return model.openExternalEditorCmd()
	}
	if key.Matches(message, model.keys.OpenURLEdit) {
		return model.openCurrentTaskURLCmd()
	}
	previous := model.notesEditor.Value()
	updatedEditor, cmd := model.notesEditor.Update(message)
	model.notesEditor = updatedEditor
	if previous != model.notesEditor.Value() {
		model.editDraft.Notes = model.notesEditor.Value()
		model.dirty = true
	}
	return cmd
}

func (model *Model) updateSubtasksMode(message tea.KeyMsg) tea.Cmd {
	if model.subtaskInputMode != "" {
		if key.Matches(message, model.keys.Cancel) {
			model.subtaskInputMode = ""
			model.subtaskInput.SetValue("")
			return nil
		}
		if key.Matches(message, model.keys.Apply) {
			title := strings.TrimSpace(model.subtaskInput.Value())
			if title == "" {
				return model.setToast("Title required")
			}
			requestID := model.nextRequestID(&model.pending.subtaskOpID)
			mode := model.subtaskInputMode
			model.subtaskInputMode = ""
			model.subtaskInput.SetValue("")
			if mode == "add" {
				position := len(model.subtasks)
				return subtaskCreateCmd(model.service, requestID, model.editDraft.ID, title, position)
			}
			if mode == "edit" {
				target := model.subtaskByID(model.subtaskEditTarget)
				if target == nil {
					return nil
				}
				target.Title = title
				return subtaskUpdateCmd(model.service, requestID, model.subtaskEditBase, *target)
			}
		}
		updatedInput, cmd := model.subtaskInput.Update(message)
		model.subtaskInput = updatedInput
		return cmd
	}

	if key.Matches(message, model.keys.ExitEdit) {
		model.mode = ModeEdit
		return nil
	}
	if key.Matches(message, model.keys.Up) {
		if model.subtaskIndex > 0 {
			model.subtaskIndex--
		}
		return nil
	}
	if key.Matches(message, model.keys.Down) {
		if model.subtaskIndex < len(model.subtasks)-1 {
			model.subtaskIndex++
		}
		return nil
	}
	if key.Matches(message, model.keys.QuickAdd) {
		model.subtaskInputMode = "add"
		model.subtaskInput.SetValue("")
		model.subtaskInput.Focus()
		return nil
	}
	if key.Matches(message, model.keys.Apply) {
		target := model.currentSubtask()
		if target == nil {
			return nil
		}
		model.subtaskInputMode = "edit"
		model.subtaskEditTarget = target.ID
		model.subtaskEditBase = target.UpdatedAt
		model.subtaskInput.SetValue(target.Title)
		model.subtaskInput.Focus()
		return nil
	}
	if key.Matches(message, model.keys.Cycle) {
		target := model.currentSubtask()
		if target == nil {
			return nil
		}
		targetCopy := *target
		targetCopy.Status = domain.CycleStatus(targetCopy.Status)
		targetCopy.UpdatedAt = model.now().UTC()
		requestID := model.nextRequestID(&model.pending.subtaskOpID)
		return subtaskUpdateCmd(model.service, requestID, target.UpdatedAt, targetCopy)
	}
	if key.Matches(message, model.keys.SubtaskDelete) {
		target := model.currentSubtask()
		if target == nil {
			return nil
		}
		model.pendingDeleteID = target.ID
		model.mode = ModeConfirmDelete
		return nil
	}
	return nil
}

func (model *Model) updateConfirmDeleteMode(message tea.KeyMsg) tea.Cmd {
	if key.Matches(message, model.keys.Cancel) {
		model.mode = ModeSubtasks
		model.pendingDeleteID = 0
		return nil
	}
	if message.String() == "y" || message.String() == "Y" {
		requestID := model.nextRequestID(&model.pending.subtaskOpID)
		id := model.pendingDeleteID
		model.pendingDeleteID = 0
		model.mode = ModeSubtasks
		return subtaskDeleteCmd(model.service, requestID, id)
	}
	return nil
}

func (model *Model) View() string {
	if model.windowWidth == 0 || model.windowHeight == 0 {
		return "loading..."
	}
	filterState := "filter: off"
	if model.filterActive {
		filterState = "filter: " + model.filterQuery
	}
	if model.mode == ModeFilterInput {
		filterState = "Filter: " + model.filterInput.Value()
	}

	selected, hasSelected := model.selectedTask()
	var selectedPtr *domain.TaskListItem
	if hasSelected {
		selectedPtr = &selected
	}
	header := components.RenderHeader(model.theme, model.windowWidth, model.listType, len(model.visibleIndices()), filterState, selectedPtr, model.dirty || model.lastSaveErr != nil)

	contentHeight := model.contentHeight()
	if contentHeight < 1 {
		contentHeight = 1
	}
	body := model.renderBody(contentHeight)

	footer := components.RenderFooterHelp(model.theme, model.windowWidth, model.helpText())
	if model.windowHeight < 4 {
		footer = components.RenderFooterHelp(model.theme, model.windowWidth, "q quit")
	}

	parts := []string{header, body}
	if model.toastMessage != "" {
		parts = append(parts, components.RenderToast(model.theme, model.toastMessage))
	}
	if model.windowHeight >= 3 {
		parts = append(parts, footer)
	}

	view := strings.Join(parts, "\n")
	if model.mode == ModeModalError {
		view = components.RenderModal(model.theme, model.windowWidth, model.windowHeight, model.errModal.Title, model.errModal.Body)
	}
	if model.mode == ModeConfirmDelete {
		view = components.RenderModal(model.theme, model.windowWidth, model.windowHeight, "Delete subtask?", "Press y to confirm, Esc to cancel")
	}
	return view
}

func (model *Model) renderBody(height int) string {
	narrow := model.windowWidth < 80 || model.windowHeight < 18
	if narrow {
		switch model.mode {
		case ModeEdit:
			return model.renderEditPane(model.windowWidth, height)
		case ModeNotesEdit:
			return components.RenderNotesEditor(model.theme, model.windowWidth, height, model.notesEditor.View())
		case ModeSubtasks:
			return model.renderSubtasks(model.windowWidth, height)
		case ModeQuickAdd:
			prompt := "New task: " + model.quickAddInput.View()
			listHeight := maxInt(1, height-1)
			return components.RenderTaskList(model.theme, model.windowWidth, listHeight, model.tasks, model.visibleIndices(), model.selectedIndex) + "\n" + prompt
		default:
			if model.mode == ModeFilterInput {
				line := "Filter: " + model.filterInput.View()
				listHeight := maxInt(1, height-1)
				return line + "\n" + components.RenderTaskList(model.theme, model.windowWidth, listHeight, model.tasks, model.visibleIndices(), model.selectedIndex)
			}
			return components.RenderTaskList(model.theme, model.windowWidth, height, model.tasks, model.visibleIndices(), model.selectedIndex)
		}
	}

	leftWidth := model.windowWidth / 3
	if leftWidth < 24 {
		leftWidth = 24
	}
	if leftWidth > model.windowWidth-20 {
		leftWidth = model.windowWidth - 20
	}
	rightWidth := model.windowWidth - leftWidth - 1
	if rightWidth < 1 {
		rightWidth = 1
	}

	paneBodyHeight := maxInt(1, height-1)
	leftContent := components.RenderTaskList(model.theme, leftWidth, paneBodyHeight, model.tasks, model.visibleIndices(), model.selectedIndex)
	rightContent := model.renderRightPane(rightWidth, paneBodyHeight)
	left := model.renderPane("Tasks", leftWidth, height, leftContent, model.mode == ModeList || model.mode == ModeQuickAdd || model.mode == ModeFilterInput)
	right := model.renderPane(model.rightPaneTitle(), rightWidth, height, rightContent, model.mode != ModeList && model.mode != ModeQuickAdd && model.mode != ModeFilterInput)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (model *Model) renderRightPane(width, height int) string {
	switch model.mode {
	case ModeEdit:
		return model.renderEditPane(width, height)
	case ModeNotesEdit:
		return components.RenderNotesEditor(model.theme, width, height, model.notesEditor.View())
	case ModeSubtasks:
		return model.renderSubtasks(width, height)
	case ModeQuickAdd:
		return "New task: " + model.quickAddInput.View()
	case ModeFilterInput:
		return "Filter: " + model.filterInput.View()
	default:
		if _, ok := model.selectedTask(); !ok {
			return model.renderEmptyDetails(width, height)
		}
		notes := model.viewport.View()
		if strings.TrimSpace(notes) == "" {
			notes = model.details.Notes
		}
		return components.RenderDetailView(model.theme, width, height, model.details, model.subtasks, notes, model.now())
	}
}

func (model *Model) renderEditPane(width, height int) string {
	urlValue := model.renderURLFieldValue(width)
	values := map[string]string{
		"title":    model.renderInputFieldValue(FieldTitle, model.form.title, false),
		"status":   fmt.Sprintf("%s %s", domain.StatusGlyph(model.editDraft.Status), domain.StatusLabel(model.editDraft.Status)),
		"url":      urlValue,
		"due":      model.renderInputFieldValue(FieldDue, model.form.due, true),
		"priority": model.renderInputFieldValue(FieldPriority, model.form.priority, true),
		"estimate": model.renderInputFieldValue(FieldEstimate, model.form.estimate, true),
		"progress": model.renderInputFieldValue(FieldProgress, model.form.progress, true),
	}
	return components.RenderEditorForm(model.theme, width, height, model.focusedFieldName(), values, len(model.subtasks))
}

func (model *Model) renderSubtasks(width, height int) string {
	lines := make([]string, 0, height)
	lines = append(lines, "Subtasks")
	for index, subtask := range model.subtasks {
		line := fmt.Sprintf("%s %s", domain.StatusGlyph(subtask.Status), subtask.Title)
		if index == model.subtaskIndex {
			line = lipgloss.NewStyle().Background(model.theme.SelectionBG).Foreground(model.theme.SelectionFG).Render(line)
		}
		lines = append(lines, line)
	}
	if model.subtaskInputMode != "" {
		lines = append(lines, "", "Title: "+model.subtaskInput.View())
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (model *Model) rightPaneTitle() string {
	switch model.mode {
	case ModeEdit:
		return "Edit"
	case ModeNotesEdit:
		return "Notes"
	case ModeSubtasks:
		return "Subtasks"
	case ModeQuickAdd:
		return "Quick Add"
	case ModeFilterInput:
		return "Filter"
	default:
		return "Details"
	}
}

func (model *Model) renderPane(title string, width, height int, content string, active bool) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	headerStyle := lipgloss.NewStyle().
		Width(width).
		Bold(true).
		Foreground(model.theme.HeaderFG).
		Background(model.theme.SelectionBG)
	if active {
		headerStyle = headerStyle.Foreground(model.theme.SelectionFG).Background(model.theme.Accent)
	}
	header := headerStyle.Render(" " + title)
	body := lipgloss.NewStyle().
		Width(width).
		Height(maxInt(0, height-1)).
		Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (model *Model) renderEmptyDetails(width, height int) string {
	lines := []string{
		lipgloss.NewStyle().Foreground(model.theme.Muted).Bold(true).Render("No task selected"),
		"",
		"Press a to create a task.",
		"Select a task and press Enter to edit.",
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (model *Model) helpText() string {
	switch model.mode {
	case ModeEdit:
		return "tab/shift+tab fields • ctrl+t/u/s/d/p/e/r/b/n jump • ctrl+space cycle • ctrl+o open URL • ctrl+k clear URL • esc back"
	case ModeNotesEdit:
		return "typing edits notes • ctrl+e external edit • ctrl+o open URL • esc back"
	case ModeSubtasks:
		return "j/k move • a add • enter edit • space cycle • x delete • esc back"
	case ModeQuickAdd:
		return "Enter create • Esc cancel"
	case ModeFilterInput:
		return "Enter apply • Esc clear"
	default:
		return "j/k move • Enter edit • a add • Space cycle • o open URL • / filter • I/U/A/C/@ lists • q quit"
	}
}

func (model *Model) rightPaneWidth() int {
	if model.windowWidth < 80 || model.windowHeight < 18 {
		return model.windowWidth
	}
	leftWidth := model.windowWidth / 3
	if leftWidth < 24 {
		leftWidth = 24
	}
	if leftWidth > model.windowWidth-20 {
		leftWidth = model.windowWidth - 20
	}
	return model.windowWidth - leftWidth - 1
}

func (model *Model) contentHeight() int {
	height := model.windowHeight - 2
	if model.toastMessage != "" {
		height--
	}
	if model.windowHeight < 4 {
		height = model.windowHeight - 1
	}
	if height < 1 {
		height = 1
	}
	return height
}

func (model *Model) reloadListCmd() tea.Cmd {
	requestID := model.nextRequestID(&model.pending.listID)
	listType := model.listType
	now := model.now()
	return func() tea.Msg {
		items, err := model.service.List(listType, now)
		return listLoadedMsg{requestID: requestID, items: items, err: err}
	}
}

func (model *Model) loadDetailsCmd(taskID int64) tea.Cmd {
	requestID := model.nextRequestID(&model.pending.detailsID)
	return func() tea.Msg {
		task, subtasks, err := model.service.GetDetails(taskID)
		return detailsLoadedMsg{requestID: requestID, task: task, subtasks: subtasks, err: err}
	}
}

func quickAddTaskCmd(svc Service, requestID int, title string) tea.Cmd {
	return func() tea.Msg {
		task, err := svc.QuickAdd(title)
		return quickAddDoneMsg{requestID: requestID, task: task, err: err}
	}
}

func cycleTaskStatusCmd(svc Service, requestID int, taskID int64, baseUpdatedAt time.Time) tea.Cmd {
	return func() tea.Msg {
		updated, err := svc.CycleTaskStatus(taskID, baseUpdatedAt)
		return statusCycleMsg{requestID: requestID, taskID: taskID, updated: updated, err: err}
	}
}

func saveTaskCmd(svc Service, requestID int, baseUpdatedAt time.Time, task domain.Task) tea.Cmd {
	return func() tea.Msg {
		updated, err := svc.UpdateTask(baseUpdatedAt, task)
		return saveResultMsg{requestID: requestID, task: updated, err: err}
	}
}

func subtaskCreateCmd(svc Service, requestID int, taskID int64, title string, position int) tea.Cmd {
	return func() tea.Msg {
		_, err := svc.CreateSubtask(taskID, title, position)
		return subtaskResultMsg{requestID: requestID, err: err}
	}
}

func subtaskUpdateCmd(svc Service, requestID int, baseUpdatedAt time.Time, subtask domain.Subtask) tea.Cmd {
	return func() tea.Msg {
		_, err := svc.UpdateSubtask(baseUpdatedAt, subtask)
		return subtaskResultMsg{requestID: requestID, err: err}
	}
}

func subtaskDeleteCmd(svc Service, requestID int, id int64) tea.Cmd {
	return func() tea.Msg {
		err := svc.DeleteSubtask(id)
		return subtaskResultMsg{requestID: requestID, err: err}
	}
}

func (model *Model) nextRequestID(target *int) int {
	model.pending.nextID++
	*target = model.pending.nextID
	return model.pending.nextID
}

func (model *Model) moveSelection(delta int) {
	visible := model.visibleIndices()
	if len(visible) == 0 {
		model.selectedIndex = 0
		return
	}
	model.selectedIndex += delta
	if model.selectedIndex < 0 {
		model.selectedIndex = 0
	}
	if model.selectedIndex >= len(visible) {
		model.selectedIndex = len(visible) - 1
	}
}

func (model *Model) clampSelection() {
	visible := model.visibleIndices()
	if len(visible) == 0 {
		model.selectedIndex = 0
		model.selectedTaskID = 0
		return
	}
	if model.selectedIndex >= len(visible) {
		model.selectedIndex = len(visible) - 1
	}
	if model.selectedIndex < 0 {
		model.selectedIndex = 0
	}
	model.selectedTaskID = model.tasks[visible[model.selectedIndex]].ID
}

func (model *Model) selectedTask() (domain.TaskListItem, bool) {
	visible := model.visibleIndices()
	if len(visible) == 0 || len(model.tasks) == 0 {
		return domain.TaskListItem{}, false
	}
	if model.selectedIndex < 0 || model.selectedIndex >= len(visible) {
		return domain.TaskListItem{}, false
	}
	return model.tasks[visible[model.selectedIndex]], true
}

func (model *Model) taskIndexByID(taskID int64) int {
	for index := range model.tasks {
		if model.tasks[index].ID == taskID {
			return index
		}
	}
	return -1
}

func (model *Model) selectTaskByID(taskID int64) {
	visible := model.visibleIndices()
	for visibleIndex, taskIndex := range visible {
		if model.tasks[taskIndex].ID == taskID {
			model.selectedIndex = visibleIndex
			model.selectedTaskID = taskID
			return
		}
	}
}

func (model *Model) visibleIndices() []int {
	if model.filterActive {
		return model.filteredIdx
	}
	indices := make([]int, len(model.tasks))
	for index := range model.tasks {
		indices[index] = index
	}
	return indices
}

func (model *Model) applyCurrentFilter() {
	query := strings.TrimSpace(model.filterQuery)
	if query == "" {
		model.filterActive = false
		model.filteredIdx = nil
		model.selectedIndex = 0
		return
	}
	indices := make([]int, 0, len(model.tasks))
	for index, task := range model.tasks {
		if fuzzyMatch(query, task.Title) {
			indices = append(indices, index)
		}
	}
	model.filterActive = true
	model.filteredIdx = indices
	if model.selectedIndex >= len(indices) {
		model.selectedIndex = 0
	}
}

func fuzzyMatch(query, candidate string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	candidate = strings.ToLower(candidate)
	if query == "" {
		return true
	}
	position := 0
	for _, ch := range candidate {
		if position < len(query) && rune(query[position]) == ch {
			position++
			if position == len(query) {
				return true
			}
		}
	}
	return false
}

func (model *Model) syncFormWithTask(task domain.Task) {
	model.editDraft = task
	model.baseUpdatedAt = task.UpdatedAt
	model.form.title.SetValue(task.Title)
	model.form.url.SetValue(task.URL)
	if task.DueAt == nil {
		model.form.due.SetValue("")
	} else {
		model.form.due.SetValue(task.DueAt.In(time.Local).Format(domain.DueDateTimeLayout))
	}
	if task.Priority == nil {
		model.form.priority.SetValue("")
	} else {
		model.form.priority.SetValue(strconv.Itoa(*task.Priority))
	}
	if task.EstimatedMinutes == nil {
		model.form.estimate.SetValue("")
	} else {
		model.form.estimate.SetValue(strconv.Itoa(*task.EstimatedMinutes))
	}
	if task.Progress == nil {
		model.form.progress.SetValue("")
	} else {
		model.form.progress.SetValue(strconv.Itoa(*task.Progress))
	}
	model.notesEditor.SetValue(task.Notes)
	model.dirty = false
}

func (model *Model) focusField() {
	model.blurFields()
	switch model.form.focused {
	case FieldTitle:
		model.form.title.Focus()
	case FieldURL:
		model.form.url.Focus()
	case FieldDue:
		model.form.due.Focus()
	case FieldPriority:
		model.form.priority.Focus()
	case FieldEstimate:
		model.form.estimate.Focus()
	case FieldProgress:
		model.form.progress.Focus()
	}
}

func (model *Model) blurFields() {
	model.form.title.Blur()
	model.form.url.Blur()
	model.form.due.Blur()
	model.form.priority.Blur()
	model.form.estimate.Blur()
	model.form.progress.Blur()
}

func (model *Model) focusNextField() {
	position := 0
	for index, field := range fieldOrder {
		if field == model.form.focused {
			position = index
			break
		}
	}
	position = (position + 1) % len(fieldOrder)
	model.form.focused = fieldOrder[position]
	model.focusField()
}

func (model *Model) focusPrevField() {
	position := 0
	for index, field := range fieldOrder {
		if field == model.form.focused {
			position = index
			break
		}
	}
	position = (position - 1 + len(fieldOrder)) % len(fieldOrder)
	model.form.focused = fieldOrder[position]
	model.focusField()
}

func (model *Model) focusedFieldName() string {
	switch model.form.focused {
	case FieldTitle:
		return "title"
	case FieldStatus:
		return "status"
	case FieldURL:
		return "url"
	case FieldDue:
		return "due"
	case FieldPriority:
		return "priority"
	case FieldEstimate:
		return "estimate"
	case FieldProgress:
		return "progress"
	case FieldSubtasks:
		return "subtasks"
	case FieldNotes:
		return "notes"
	default:
		return "title"
	}
}

func (model *Model) isTextInputFieldFocused() bool {
	switch model.form.focused {
	case FieldTitle, FieldURL, FieldDue, FieldPriority, FieldEstimate, FieldProgress:
		return true
	default:
		return false
	}
}

func (model *Model) matchesClearURL(message tea.KeyMsg) bool {
	if key.Matches(message, model.keys.ClearURL) || message.Type == tea.KeyCtrlK {
		return true
	}
	return message.Type == tea.KeyRunes && len(message.Runes) == 1 && message.Runes[0] == 0x0b
}

func (model *Model) renderURLFieldValue(width int) string {
	const clearHint = " (ctrl+k clear)"
	if model.form.focused == FieldURL {
		return model.form.url.View() + clearHint
	}
	available := width - len("URL: ") - len(clearHint)
	if available < 10 {
		available = 10
	}
	return components.CompactURL(model.form.url.Value(), available) + clearHint
}

func (model *Model) renderInputFieldValue(field FieldFocus, input textinput.Model, showNone bool) string {
	if model.form.focused == field {
		return input.View()
	}
	if showNone {
		return valueOrNoneString(input.Value())
	}
	return input.Value()
}

func (model *Model) updateFocusedInput(message tea.KeyMsg) tea.Cmd {
	switch model.form.focused {
	case FieldTitle:
		updated, cmd := model.form.title.Update(message)
		model.form.title = updated
		model.editDraft.Title = model.form.title.Value()
		return cmd
	case FieldURL:
		updated, cmd := model.form.url.Update(message)
		model.form.url = updated
		model.editDraft.URL = model.form.url.Value()
		return cmd
	case FieldDue:
		updated, cmd := model.form.due.Update(message)
		model.form.due = updated
		return cmd
	case FieldPriority:
		updated, cmd := model.form.priority.Update(message)
		model.form.priority = updated
		return cmd
	case FieldEstimate:
		updated, cmd := model.form.estimate.Update(message)
		model.form.estimate = updated
		return cmd
	case FieldProgress:
		updated, cmd := model.form.progress.Update(message)
		model.form.progress = updated
		return cmd
	default:
		return nil
	}
}

func (model *Model) formValuesHash() string {
	return strings.Join([]string{
		model.form.title.Value(),
		model.form.url.Value(),
		model.form.due.Value(),
		model.form.priority.Value(),
		model.form.estimate.Value(),
		model.form.progress.Value(),
		model.editDraft.Status,
		model.notesEditor.Value(),
	}, "\x00")
}

func (model *Model) startSaveCmd() tea.Cmd {
	if model.saveInFlight {
		return nil
	}
	task, field, err := model.buildTaskFromForm()
	if err != nil {
		if field != "" {
			return model.setToast("Invalid " + field)
		}
		return model.setToast("Autosave failed")
	}
	model.saveInFlight = true
	requestID := model.nextRequestID(&model.pending.saveID)
	return saveTaskCmd(model.service, requestID, model.baseUpdatedAt, task)
}

func (model *Model) buildTaskFromForm() (domain.Task, string, error) {
	task := model.editDraft
	task.Title = strings.TrimSpace(model.form.title.Value())
	task.URL = strings.TrimSpace(model.form.url.Value())
	task.Notes = model.notesEditor.Value()

	dueAt, err := domain.ParseDueInput(model.form.due.Value(), time.Local)
	if err != nil {
		return domain.Task{}, "due", err
	}
	task.DueAt = dueAt

	priority, err := parseOptionalBoundedInt(model.form.priority.Value(), 1, 4)
	if err != nil {
		return domain.Task{}, "priority", err
	}
	task.Priority = priority

	estimate, err := domain.ParseEstimatedMinutes(model.form.estimate.Value())
	if err != nil {
		return domain.Task{}, "estimated time", err
	}
	task.EstimatedMinutes = estimate

	progress, err := parseOptionalBoundedInt(model.form.progress.Value(), 0, 100)
	if err != nil {
		return domain.Task{}, "progress", err
	}
	task.Progress = progress

	if task.Meta == nil {
		task.Meta = map[string]any{}
	}
	return task, "", nil
}

func parseOptionalBoundedInt(input string, minValue, maxValue int) (*int, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil, err
	}
	if value < minValue || value > maxValue {
		return nil, fmt.Errorf("out of range")
	}
	return &value, nil
}

func (model *Model) setToast(message string) tea.Cmd {
	model.toastMessage = message
	expiry := model.now().Add(2 * time.Second)
	model.toastExpiry = expiry
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return toastClearMsg{time: expiry}
	})
}

func (model *Model) autosaveTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return autosaveTickMsg{time: t}
	})
}

func (model *Model) openCurrentTaskURLCmd() tea.Cmd {
	urlValue := strings.TrimSpace(model.editDraft.URL)
	if urlValue == "" {
		urlValue = strings.TrimSpace(model.details.URL)
	}
	if urlValue == "" {
		return model.setToast("No URL")
	}
	normalized, err := domain.NormalizeURL(urlValue)
	if err != nil {
		return model.setToast("Invalid URL")
	}
	return OpenURLCmd(normalized)
}

func (model *Model) openExternalEditorCmd() tea.Cmd {
	current := model.notesEditor.Value()
	editor := strings.TrimSpace(model.editor)
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vim"
	}
	return OpenEditorForNotesCmd(editor, current)
}

func ExitAltScreenThen(cmd tea.Cmd) tea.Cmd {
	return cmd
}

func OpenURLCmd(url string) tea.Cmd {
	return ExitAltScreenThen(func() tea.Msg {
		var command *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			command = exec.Command("open", url)
		case "windows":
			command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			command = exec.Command("xdg-open", url)
		}
		if err := command.Run(); err != nil {
			return urlOpenResultMsg{err: fmt.Errorf("open URL failed: %w", err)}
		}
		return urlOpenResultMsg{}
	})
}

func OpenEditorForNotesCmd(editor string, currentNotes string) tea.Cmd {
	return ExitAltScreenThen(func() tea.Msg {
		temporaryFile, err := os.CreateTemp("", "bucket-notes-*.md")
		if err != nil {
			return notesExternalEditMsg{err: err}
		}
		path := temporaryFile.Name()
		defer os.Remove(path)
		if _, err := temporaryFile.WriteString(currentNotes); err != nil {
			_ = temporaryFile.Close()
			return notesExternalEditMsg{err: err}
		}
		if err := temporaryFile.Close(); err != nil {
			return notesExternalEditMsg{err: err}
		}

		parts := strings.Fields(editor)
		if len(parts) == 0 {
			parts = []string{"vim"}
		}
		command := exec.Command(parts[0], append(parts[1:], path)...)
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			return notesExternalEditMsg{err: err}
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return notesExternalEditMsg{err: err}
		}
		return notesExternalEditMsg{notes: string(payload)}
	})
}

func (model *Model) flushDirty(timeout time.Duration) error {
	if !model.dirty || model.editDraft.ID == 0 {
		return nil
	}
	task, _, err := model.buildTaskFromForm()
	if err != nil {
		_, _ = model.writeDraftNow()
		return err
	}
	result := make(chan error, 1)
	base := model.baseUpdatedAt
	go func() {
		_, updateErr := model.service.UpdateTask(base, task)
		result <- updateErr
	}()
	select {
	case err := <-result:
		if err != nil {
			_, _ = model.writeDraftNow()
			return err
		}
		model.dirty = false
		_ = DeleteDraftFile(model.draftsDir, task.ID)
		return nil
	case <-time.After(timeout):
		_, _ = model.writeDraftNow()
		return context.DeadlineExceeded
	}
}

func (model *Model) handleQuit() tea.Cmd {
	_ = model.flushDirty(300 * time.Millisecond)
	if model.dirty {
		_, _ = model.writeDraftNow()
	}
	return tea.Quit
}

func (model *Model) currentSubtask() *domain.Subtask {
	if model.subtaskIndex < 0 || model.subtaskIndex >= len(model.subtasks) {
		return nil
	}
	return &model.subtasks[model.subtaskIndex]
}

func (model *Model) subtaskByID(id int64) *domain.Subtask {
	for index := range model.subtasks {
		if model.subtasks[index].ID == id {
			return &model.subtasks[index]
		}
	}
	return nil
}

// handleFieldSelectors provides quick selectors for certain fields when focused.
// Returns true if it handled the key (preventing further processing).
func (model *Model) handleFieldSelectors(message tea.KeyMsg) bool {
	switch model.form.focused {
	case FieldPriority:
		if message.Type == tea.KeyUp || message.Type == tea.KeyDown {
			model.cyclePriority(message.Type == tea.KeyUp)
			return true
		}
	case FieldEstimate:
		if message.Type == tea.KeyUp || message.Type == tea.KeyDown {
			model.bumpEstimate(message.Type == tea.KeyUp)
			return true
		}
	case FieldDue:
		if message.Type == tea.KeyUp || message.Type == tea.KeyDown {
			model.shiftDueDate(message.Type == tea.KeyUp)
			return true
		}
	}
	return false
}

func (model *Model) cyclePriority(increase bool) {
	current := strings.TrimSpace(model.form.priority.Value())
	val, err := strconv.Atoi(current)
	if err != nil || val < 1 || val > 4 {
		if increase {
			val = 1
		} else {
			val = 4
		}
	} else {
		if increase {
			val++
			if val > 4 {
				val = 0
			}
		} else {
			val--
			if val < 1 {
				val = 0
			}
		}
	}
	if val == 0 {
		model.form.priority.SetValue("")
		model.editDraft.Priority = nil
	} else {
		model.form.priority.SetValue(strconv.Itoa(val))
		model.editDraft.Priority = &val
	}
	model.dirty = true
}

func (model *Model) bumpEstimate(increase bool) {
	current := strings.TrimSpace(model.form.estimate.Value())
	minutesPtr, _ := domain.ParseEstimatedMinutes(current)
	var minutes int
	if minutesPtr == nil {
		if increase {
			minutes = 30
		} else {
			return
		}
	} else {
		minutes = *minutesPtr
		if increase {
			minutes += 15
		} else {
			minutes -= 15
		}
		if minutes < 1 {
			minutes = 0
		}
		if minutes > 100000 {
			minutes = 100000
		}
	}
	if minutes == 0 {
		model.form.estimate.SetValue("")
		model.editDraft.EstimatedMinutes = nil
	} else {
		model.form.estimate.SetValue(strconv.Itoa(minutes))
		model.editDraft.EstimatedMinutes = &minutes
	}
	model.dirty = true
}

func (model *Model) shiftDueDate(increase bool) {
	current := strings.TrimSpace(model.form.due.Value())
	parsed, _ := domain.ParseDueInput(current, time.Local)
	var base time.Time
	if parsed == nil {
		base = time.Now().In(time.Local)
		base = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, time.Local)
	} else {
		base = parsed.In(time.Local)
	}
	if increase {
		base = base.AddDate(0, 0, 1)
	} else {
		base = base.AddDate(0, 0, -1)
	}
	model.form.due.SetValue(base.Format(domain.DueDateLayout))
	model.dirty = true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func valueOrNoneString(input string) string {
	if strings.TrimSpace(input) == "" {
		return "(none)"
	}
	return input
}

// Draft persistence.

type TaskDraft struct {
	TaskID        int64           `json:"task_id"`
	BaseUpdatedAt int64           `json:"base_updated_at"`
	SavedAt       int64           `json:"saved_at"`
	Fields        TaskDraftFields `json:"fields"`
	Meta          map[string]any  `json:"meta"`
}

type TaskDraftFields struct {
	Title    string `json:"title"`
	Status   string `json:"status"`
	URL      string `json:"url"`
	Notes    string `json:"notes"`
	Due      string `json:"due"`
	Priority string `json:"priority"`
	Estimate string `json:"estimate"`
	Progress string `json:"progress"`
}

func DraftFilePath(draftsDir string, taskID int64) string {
	return filepath.Join(draftsDir, fmt.Sprintf("task-%d.json", taskID))
}

func (model *Model) writeDraftNow() (string, error) {
	if model.editDraft.ID == 0 {
		return "", nil
	}
	draft := TaskDraft{
		TaskID:        model.editDraft.ID,
		BaseUpdatedAt: model.baseUpdatedAt.UTC().Unix(),
		SavedAt:       model.now().UTC().Unix(),
		Fields: TaskDraftFields{
			Title:    model.form.title.Value(),
			Status:   model.editDraft.Status,
			URL:      model.form.url.Value(),
			Notes:    model.notesEditor.Value(),
			Due:      model.form.due.Value(),
			Priority: model.form.priority.Value(),
			Estimate: model.form.estimate.Value(),
			Progress: model.form.progress.Value(),
		},
		Meta: model.editDraft.Meta,
	}
	return WriteDraftFile(model.draftsDir, draft)
}

func WriteDraftFile(draftsDir string, draft TaskDraft) (string, error) {
	if draft.TaskID == 0 {
		return "", errors.New("draft task id is required")
	}
	if err := os.MkdirAll(draftsDir, 0o700); err != nil {
		return "", fmt.Errorf("create drafts dir %s: %w", draftsDir, err)
	}
	payload, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode draft: %w", err)
	}
	path := DraftFilePath(draftsDir, draft.TaskID)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return "", fmt.Errorf("write draft %s: %w", path, err)
	}
	return path, nil
}

func DeleteDraftFile(draftsDir string, taskID int64) error {
	if taskID == 0 {
		return nil
	}
	path := DraftFilePath(draftsDir, taskID)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func LoadDraftFiles(draftsDir string) ([]TaskDraft, []string, error) {
	entries, err := os.ReadDir(draftsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read drafts dir %s: %w", draftsDir, err)
	}
	drafts := make([]TaskDraft, 0, len(entries))
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "task-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(draftsDir, entry.Name())
		payload, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read draft %s: %w", path, err)
		}
		var draft TaskDraft
		if err := json.Unmarshal(payload, &draft); err != nil {
			return nil, nil, fmt.Errorf("parse draft %s: %w", path, err)
		}
		drafts = append(drafts, draft)
		paths = append(paths, path)
	}
	sort.SliceStable(drafts, func(i, j int) bool {
		return drafts[i].TaskID < drafts[j].TaskID
	})
	return drafts, paths, nil
}

func ApplyDraftToTask(base domain.Task, draft TaskDraft) (domain.Task, error) {
	if base.ID != draft.TaskID {
		return domain.Task{}, fmt.Errorf("draft task id %d does not match base task %d", draft.TaskID, base.ID)
	}
	base.Title = draft.Fields.Title
	base.Status = draft.Fields.Status
	base.URL = draft.Fields.URL
	base.Notes = draft.Fields.Notes
	base.Meta = draft.Meta

	dueAt, err := domain.ParseDueInput(draft.Fields.Due, time.Local)
	if err != nil {
		return domain.Task{}, err
	}
	base.DueAt = dueAt

	priority, err := parseOptionalBoundedInt(draft.Fields.Priority, 1, 4)
	if err != nil {
		return domain.Task{}, err
	}
	base.Priority = priority

	estimated, err := domain.ParseEstimatedMinutes(draft.Fields.Estimate)
	if err != nil {
		return domain.Task{}, err
	}
	base.EstimatedMinutes = estimated

	progress, err := parseOptionalBoundedInt(draft.Fields.Progress, 0, 100)
	if err != nil {
		return domain.Task{}, err
	}
	base.Progress = progress
	return base, nil
}
