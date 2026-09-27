package replyclassify

import "testing"

// A declined interest carries "interested", which the interest phrases would
// read as interest and send down a positive-reply branch.
func TestLexiconDeclinedInterest(t *testing.T) {
	cases := map[string]string{
		"Not interested.":                                   ClassNegative,
		"I'm not interested, thanks.":                       ClassNegative,
		"We are not interested at this time.":               ClassNegative,
		"Thanks, but we're not really interested.":          ClassNegative,
		"Not currently interested.":                         ClassNegative,
		"No longer interested.":                             ClassNegative,
		"We would not be interested.":                       ClassNegative,
		"We wouldn\u2019t be interested.":                   ClassNegative,
		"We aren't interested.":                             ClassNegative,
		"Honestly, not that interested.":                    ClassNegative,
		"Uninterested.":                                     ClassNegative,
		"Not interested right now, tell me more next year.": ClassNegative,
		"Not interested, please unsubscribe me.":            ClassUnsubscribe,
		"Interested!":                                       ClassPositive,
		"Very interested, let's talk.":                      ClassPositive,
		"I'd be interested to hear more.":                   ClassPositive,
		"No thanks.":                                        ClassNegative,
	}
	for body, want := range cases {
		r, ok := classifyLexicon(Input{BodyText: body})
		if !ok || r.Class != want {
			t.Errorf("classifyLexicon(%q) = %q (%v), want %q", body, r.Class, ok, want)
		}
	}
}
