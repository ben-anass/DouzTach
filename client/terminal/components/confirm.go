package components

import (
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/styles"
)

// Confirm describes a yes/no confirmation before a destructive or irreversible
// action.
type Confirm struct {
	Title       string
	Question    string
	Detail      string
	ConfirmKey  string
	CancelKey   string
	Destructive bool
}

// NewConfirm builds a confirmation with safe defaults.
func NewConfirm(title, question string) Confirm {
	return Confirm{
		Title:      title,
		Question:   question,
		ConfirmKey: "y",
		CancelKey:  "n",
	}
}

// RenderConfirm renders the confirmation modal centred in the terminal.
func RenderConfirm(s styles.Styles, confirm Confirm, width, height int) string {
	body := confirm.Question
	if detail := strings.TrimSpace(confirm.Detail); detail != "" {
		body += "\n\n" + detail
	}
	tone := ToneNeutral
	if confirm.Destructive {
		tone = ToneError
	}
	return RenderModal(s, Modal{
		Title: confirm.Title,
		Body:  body,
		Tone:  tone,
		Actions: []ModalAction{
			{Label: "Confirm", Key: confirm.ConfirmKey, Primary: !confirm.Destructive, Destructive: confirm.Destructive},
			{Label: "Cancel", Key: confirm.CancelKey},
		},
	}, width, height)
}

// ParseConfirmation interprets a typed confirmation answer. It accepts only
// explicit yes/no values so an accidental keypress cannot confirm a destructive
// action.
func ParseConfirmation(answer string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	case "n", "no", "":
		return false, nil
	default:
		return false, fmt.Errorf("please answer yes or no")
	}
}
