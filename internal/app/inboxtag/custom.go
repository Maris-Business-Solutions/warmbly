package inboxtag

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/warmbly/warmbly/internal/models"
)

// Workspace questions ride in the same call as the built-in set. They add
// labels and may act through the same Plan, but never touch kind, intent or
// relevance: those stay calibrated against the fixtures.

// customPrefix keys a workspace question's answer, so it can never collide
// with a built-in id in the one question map.
const customPrefix = "custom_"

// noneOfThese is added to every choice question, so a reply that fits no
// option has somewhere to go instead of being forced into a wrong label.
const noneOfThese = "none_of_these"

// CustomMatch is one workspace question that fired on a message.
type CustomMatch struct {
	QuestionID string
	Label      string
	Action     models.InboxTagQuestionAction
	// Strength is the noul, or the choice confidence, it fired at.
	Strength float64
	// Acts is whether the answer is strong enough to act on, not only label:
	// Strong for a yes/no, ConfFloor for a choice.
	Acts bool
}

func customKey(id string) string { return customPrefix + id }

func optionKey(i int) string { return "option_" + strconv.Itoa(i+1) }

// QuestionsFor is the built-in set plus the workspace's own, and the
// action-required check when the workspace keeps such mail in the inbox.
func QuestionsFor(custom []models.InboxTagQuestion, actionRequired bool) map[string]Question {
	q := Questions()
	if actionRequired {
		q[SigActionRequired] = ActionRequiredQuestion()
	}
	addCustom(q, custom)
	return q
}

// NotificationQuestions is what is asked of a notification decided offline:
// the action-required check and the workspace questions asked of automated
// mail. Empty means nothing is worth a call.
func NotificationQuestions(custom []models.InboxTagQuestion, actionRequired bool) map[string]Question {
	q := map[string]Question{}
	if actionRequired {
		q[SigActionRequired] = ActionRequiredQuestion()
	}
	var own []models.InboxTagQuestion
	for _, c := range custom {
		if c.Automated {
			own = append(own, c)
		}
	}
	addCustom(q, own)
	return q
}

func addCustom(q map[string]Question, custom []models.InboxTagQuestion) {
	for _, c := range custom {
		switch c.Type {
		case models.InboxTagQuestionYesNo:
			q[customKey(c.ID)] = Question{Type: QuestionNoul, Instructions: c.Question}
		case models.InboxTagQuestionChoice:
			criteria := make(map[string]string, len(c.Choices)+1)
			for i, opt := range c.Choices {
				criteria[optionKey(i)] = opt.Description
			}
			criteria[noneOfThese] = "None of the other options fits this message"
			q[customKey(c.ID)] = Question{Type: QuestionChoice, Instructions: c.Question, Criteria: criteria}
		}
	}
}

// decideCustom reads the workspace questions' answers onto a decision. A
// question is about what someone said, so on mail no person wrote only one
// the workspace asked of notifications applies, and a match keeps it in the
// inbox.
func decideCustom(d *Decision, answers map[string]Answer, custom []models.InboxTagQuestion) {
	if d.Kind == "" {
		return
	}
	automated := IsAutomatedKind(d.Kind)
	seen := map[string]bool{}
	for _, l := range d.Labels {
		seen[l] = true
	}
	add := func(m CustomMatch) {
		d.Custom = append(d.Custom, m)
		if automated {
			d.KeepInInbox = true
		}
		if !seen[m.Label] {
			seen[m.Label] = true
			d.Labels = append(d.Labels, m.Label)
		}
	}
	for _, c := range custom {
		if automated && (!c.Automated || d.Kind != KindNotification) {
			continue
		}
		a, ok := answers[customKey(c.ID)]
		if !ok {
			continue
		}
		switch c.Type {
		case models.InboxTagQuestionYesNo:
			if a.Noul >= Yes && c.Label != "" {
				add(CustomMatch{QuestionID: c.ID, Label: c.Label, Action: c.Action, Strength: a.Noul, Acts: a.Noul >= Strong})
			}
		case models.InboxTagQuestionChoice:
			if a.Confidence < ConfFloor || a.Choice == noneOfThese {
				continue
			}
			for i, opt := range c.Choices {
				if a.Choice == optionKey(i) && opt.Label != "" {
					add(CustomMatch{QuestionID: c.ID, Label: opt.Label, Action: opt.Action, Strength: a.Confidence, Acts: true})
					break
				}
			}
		}
	}
}

// planCustom folds the workspace questions' actions into a plan: the longest
// hold wins, a stop is a stop, and a built-in task keeps its own title.
func planCustom(p *Plan, d Decision) {
	for _, m := range d.Custom {
		if !m.Acts {
			continue
		}
		switch m.Action.Type {
		case models.InboxTagActionHold:
			if m.Action.HoldDays > p.HoldDays {
				p.HoldDays = m.Action.HoldDays
				p.HoldReason = fmt.Sprintf("replied %q", m.Label)
			}
		case models.InboxTagActionStop:
			p.Stop = true
		case models.InboxTagActionTask:
			if p.Task == "" {
				p.Task = fmt.Sprintf("Reply tagged %q", m.Label)
			}
		}
	}
}

// ReservedLabel reports a label a workspace question may not use: one the
// built-in taxonomy already writes, in any case, which would make its meaning
// ambiguous.
func ReservedLabel(label string) bool {
	for _, l := range AllLabels() {
		if strings.EqualFold(l, label) {
			return true
		}
	}
	return false
}

// ValidateQuestions refuses a built-in label name, unless that question was already saved with it.
func ValidateQuestions(qs, saved []models.InboxTagQuestion) error {
	kept := map[string]bool{}
	for _, q := range saved {
		for _, l := range CustomLabels([]models.InboxTagQuestion{q}) {
			kept[q.ID+"/"+strings.ToLower(l)] = true
		}
	}
	for _, q := range qs {
		labels := CustomLabels([]models.InboxTagQuestion{q})
		for _, label := range labels {
			if ReservedLabel(label) && !kept[q.ID+"/"+strings.ToLower(label)] {
				return fmt.Errorf("label %q is a built-in tagging label; pick another name", label)
			}
		}
	}
	return nil
}

// CustomLabels is every label the workspace questions can write.
func CustomLabels(qs []models.InboxTagQuestion) []string {
	var out []string
	for _, q := range qs {
		if q.Label != "" {
			out = append(out, q.Label)
		}
		for _, c := range q.Choices {
			if c.Label != "" {
				out = append(out, c.Label)
			}
		}
	}
	return out
}

// LanguageHint is what the state carries for a workspace's tagging languages:
// their names, or "" when none is chosen.
func LanguageHint(codes []string) string {
	names := make([]string, 0, len(codes))
	for _, c := range codes {
		if n := models.MailLanguageNames[c]; n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, ", ")
}
