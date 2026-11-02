package prompts

import (
	"strings"
	"testing"

	"github.com/ben-anass/DouzTach/client/terminal/styles"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestForm() *Form {
	s := styles.New(true)
	return NewForm(s, "Sign in", []Field{
		{Key: "email", Label: "Email", Validate: func(value string) error {
			if !strings.Contains(value, "@") {
				return errTest("invalid email")
			}
			return nil
		}},
		{Key: "password", Label: "Password", Secret: true, Validate: RequireValue("password")},
	})
}

type errTest string

func (e errTest) Error() string { return string(e) }

func TestFormValuesTrimNonSecrets(t *testing.T) {
	t.Parallel()
	form := newTestForm()
	form.inputs[0].SetValue("  jane@example.com  ")
	form.inputs[1].SetValue(" secret pass ")
	values := form.Values()
	if values["email"] != "jane@example.com" {
		t.Fatalf("email = %q", values["email"])
	}
	if values["password"] != " secret pass " {
		t.Fatalf("password should be preserved exactly, got %q", values["password"])
	}
}

func TestFormMasksPasswordInView(t *testing.T) {
	t.Parallel()
	form := newTestForm()
	form.inputs[0].SetValue("jane@example.com")
	form.inputs[1].SetValue("super-secret-password")
	view := form.View()
	if strings.Contains(view, "super-secret-password") {
		t.Fatal("form view leaked the password")
	}
	if !strings.Contains(view, "hidden") {
		t.Fatalf("password field should be labelled hidden: %q", view)
	}
}

func TestFormRejectsInvalidField(t *testing.T) {
	t.Parallel()
	form := newTestForm()
	form.Init()
	form.inputs[0].SetValue("not-an-email")
	model, _ := form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := model.(*Form)
	if result.Submitted() {
		t.Fatal("form must not submit with an invalid field")
	}
	if result.errs[0] == nil {
		t.Fatal("expected a field error")
	}
}

func TestFormSubmitsValidValues(t *testing.T) {
	t.Parallel()
	form := newTestForm()
	form.Init()
	form.inputs[0].SetValue("jane@example.com")
	form.inputs[1].SetValue("password1")
	form.focus = 1
	form.syncFocus()
	model, _ := form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := model.(*Form)
	if !result.Submitted() {
		t.Fatalf("expected submission, errors: %v", result.errs)
	}
	if result.Value("email") != "jane@example.com" {
		t.Fatalf("email = %q", result.Value("email"))
	}
}

func TestFormSubmitValidator(t *testing.T) {
	t.Parallel()
	form := newTestForm()
	form.SubmitValidator = func(values map[string]string) error {
		if values["password"] != "expected" {
			return errTest("passwords do not match")
		}
		return nil
	}
	form.Init()
	form.inputs[0].SetValue("jane@example.com")
	form.inputs[1].SetValue("wrong")
	form.focus = 1
	form.syncFocus()
	model, _ := form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := model.(*Form)
	if result.Submitted() {
		t.Fatal("cross-field validation should block submission")
	}
	if result.SubmitError() == nil {
		t.Fatal("expected submit error")
	}
}

func TestFormEscapeCancels(t *testing.T) {
	t.Parallel()
	form := newTestForm()
	form.Init()
	model, _ := form.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := model.(*Form)
	if !result.Cancelled() {
		t.Fatal("escape should cancel the form")
	}
}

func TestSelectNavigationAndChoice(t *testing.T) {
	t.Parallel()
	selectPrompt := NewSelect(styles.New(true), "Workspace", []Option{
		{Label: "One", Value: "1"},
		{Label: "Two", Value: "2"},
		{Label: "Three", Value: "3"},
	})
	selectPrompt.Init()
	selectPrompt.Update(tea.KeyMsg{Type: tea.KeyDown})
	selectPrompt.Update(tea.KeyMsg{Type: tea.KeyDown})
	model, _ := selectPrompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := model.(*Select)
	option, ok := result.Selected()
	if !ok {
		t.Fatal("expected a selected option")
	}
	if option.Value != "3" {
		t.Fatalf("selected = %q", option.Value)
	}
}

func TestSelectSkipsDisabledOptions(t *testing.T) {
	t.Parallel()
	selectPrompt := NewSelect(styles.New(true), "Workspace", []Option{
		{Label: "Disabled", Value: "1", Disabled: true},
		{Label: "Enabled", Value: "2"},
	})
	selectPrompt.Init()
	selectPrompt.Update(tea.KeyMsg{Type: tea.KeyDown})
	if selectPrompt.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", selectPrompt.cursor)
	}
}

func TestSelectEscapeCancels(t *testing.T) {
	t.Parallel()
	selectPrompt := NewSelect(styles.New(true), "Workspace", []Option{{Label: "One", Value: "1"}})
	selectPrompt.Init()
	model, _ := selectPrompt.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !model.(*Select).Cancelled() {
		t.Fatal("escape should cancel the select prompt")
	}
}

func TestDestructiveConfirmRequiresExplicitYes(t *testing.T) {
	t.Parallel()
	confirm := NewConfirm(styles.New(true), "Delete", "Delete this task?")
	confirm.Destructive = true
	confirm.Init()
	model, _ := confirm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model.(*Confirm).Confirmed() {
		t.Fatal("enter must not confirm a destructive action")
	}
	model, _ = model.(*Confirm).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !model.(*Confirm).Confirmed() {
		t.Fatal("explicit y should confirm")
	}
}

func TestConfirmNoCancels(t *testing.T) {
	t.Parallel()
	confirm := NewConfirm(styles.New(true), "Delete", "Delete this task?")
	confirm.Destructive = true
	confirm.Init()
	model, _ := confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	result := model.(*Confirm)
	if result.Confirmed() {
		t.Fatal("n must not confirm")
	}
	if !result.Cancelled() {
		t.Fatal("n should mark the prompt cancelled")
	}
}

func TestRegisterFormPasswordMismatch(t *testing.T) {
	t.Parallel()
	form := RegisterForm(styles.New(true))
	if form.SubmitValidator == nil {
		t.Fatal("register form needs a cross-field validator")
	}
	err := form.SubmitValidator(map[string]string{
		"password":         "password1",
		"confirm_password": "password2",
	})
	if err == nil {
		t.Fatal("mismatched passwords should fail")
	}
}

func TestTaskFormOnlyAsksForFreeText(t *testing.T) {
	t.Parallel()
	form := TaskForm(styles.New(true))
	want := []string{"title", "description", "due_date", "story_points"}
	if len(form.Fields) != len(want) {
		t.Fatalf("fields = %d, want %d", len(form.Fields), len(want))
	}
	for i, key := range want {
		if form.Fields[i].Key != key {
			t.Fatalf("field %d = %q, want %q", i, form.Fields[i].Key, key)
		}
	}
	for _, key := range []string{"type", "priority"} {
		for _, field := range form.Fields {
			if field.Key == key {
				t.Fatalf("%q must be chosen with a picker, not typed", key)
			}
		}
	}
}

func TestNewSelectSelectedPreselectsValue(t *testing.T) {
	t.Parallel()
	prompt := NewSelectSelected(styles.New(true), "Priority", []Option{
		{Label: "LOW", Value: "LOW"},
		{Label: "MEDIUM", Value: "MEDIUM"},
		{Label: "HIGH", Value: "HIGH"},
	}, "HIGH")
	model, _ := prompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
	option, ok := model.(*Select).Selected()
	if !ok {
		t.Fatal("expected a selected option")
	}
	if option.Value != "HIGH" {
		t.Fatalf("selected = %q, want HIGH", option.Value)
	}
}

func TestSelectValueIgnoresUnknownValue(t *testing.T) {
	t.Parallel()
	prompt := NewSelectSelected(styles.New(true), "Status", []Option{
		{Label: "TODO", Value: "TODO"},
		{Label: "DONE", Value: "DONE"},
	}, "MISSING")
	model, _ := prompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
	option, ok := model.(*Select).Selected()
	if !ok || option.Value != "TODO" {
		t.Fatalf("selected = %+v, want the first option", option)
	}
}

func TestTaskUpdateRequiresAChange(t *testing.T) {
	t.Parallel()
	form := TaskUpdateForm(styles.New(true))
	if form.SubmitValidator == nil {
		t.Fatal("task update form needs a validator")
	}
	if err := form.SubmitValidator(form.Values()); err == nil {
		t.Fatal("an empty update should be rejected")
	}
	values := form.Values()
	values["status"] = "DONE"
	if err := form.SubmitValidator(values); err != nil {
		t.Fatalf("non-empty update should pass: %v", err)
	}
}
