package prompts

import (
	"strings"
	"testing"
)

func TestAssembleContext_AlwaysIncludesUserMessage(t *testing.T) {
	in := PersonalizedCoachingInput{UserMessage: "I need help with my morning routine"}
	// Tiny budget that drops most context — the user message must survive.
	prompt, bd := AssembleContextWithBreakdown(in, 50)
	if !strings.Contains(prompt, "User's Message: I need help with my morning routine") {
		t.Fatalf("user message missing from prompt:\n%s", prompt)
	}
	// User message is not part of the section breakdown.
	for _, s := range bd {
		if s.Section == "user_message" {
			t.Fatalf("user message should not be a budgeted section")
		}
	}
}

func TestAssembleContext_BudgetDropsLowPrioritySections(t *testing.T) {
	in := PersonalizedCoachingInput{
		UserMessage:           "hi",
		AccountabilityStyle:   "balanced",
		PreferredTone:         "supportive",
		DifficultyPreference:  "adaptive",
		ActiveGoals:           []string{"Run a marathon", "Read more books", "Learn guitar"},
		ActiveHabits:          []string{"Morning run", "Reading", "Practice guitar"},
		RecentCheckInsSummary: "Recent activity: 30 check-ins with 70.0% completion rate.",
		CommonBlockers:        []string{"too tired", "no time", "distractions"},
		PatternInsights:       map[string]string{"completion_pattern": "moderate_consistency", "top_blocker": "too tired"},
	}

	// Large budget: everything fits.
	full, fullBD := AssembleContextWithBreakdown(in, 10000)
	for _, s := range fullBD {
		if !s.Included {
			t.Fatalf("with large budget, section %s should be included", s.Section)
		}
	}
	if !strings.Contains(full, "Common Blockers") {
		t.Fatalf("large budget should include blockers")
	}

	// Tiny budget: low-priority sections (blockers=4) should be dropped.
	tight, tightBD := AssembleContextWithBreakdown(in, 80)
	gotBlockers := false
	for _, s := range tightBD {
		if s.Section == "common_blockers" {
			gotBlockers = s.Included
		}
	}
	if gotBlockers {
		t.Fatalf("tight budget should drop common_blockers (lowest priority)")
	}
	if strings.Contains(tight, "Common Blockers") {
		t.Fatalf("tight budget prompt should not contain blockers:\n%s", tight)
	}
}

func TestSelectRelevant_RanksMatchingItemsFirst(t *testing.T) {
	items := []string{"Morning run", "Read books", "Practice guitar", "Meditate"}
	// "run" matches "Morning run".
	selected, rest := selectRelevant("I want to keep up my run streak", items, 2)
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected, got %d (%v)", len(selected), selected)
	}
	if selected[0] != "Morning run" {
		t.Fatalf("expected 'Morning run' first, got %q", selected[0])
	}
	if rest != 2 {
		t.Fatalf("expected 2 remaining, got %d", rest)
	}
}

func TestSelectRelevant_NoMatchReturnsFirstK(t *testing.T) {
	items := []string{"A", "B", "C", "D"}
	selected, rest := selectRelevant("hello there", items, 2)
	if len(selected) != 2 || rest != 2 {
		t.Fatalf("expected 2 selected + 2 rest, got %d selected %d rest", len(selected), rest)
	}
}

func TestSelectRelevant_Empty(t *testing.T) {
	selected, rest := selectRelevant("anything", nil, 5)
	if selected != nil || rest != 0 {
		t.Fatalf("expected nil/0, got %v/%d", selected, rest)
	}
}

func TestRenderRelevantGoals_SummarizesRest(t *testing.T) {
	goals := make([]string, 12)
	for i := range goals {
		goals[i] = "goal"
	}
	in := PersonalizedCoachingInput{UserMessage: "hi", ActiveGoals: goals}
	out := renderRelevantGoals(in)
	if !strings.Contains(out, "(plus 7 other active goals)") {
		t.Fatalf("expected rest summary, got:\n%s", out)
	}
	// Should list exactly maxRelevantGoals items.
	if strings.Count(out, ". goal\n") != maxRelevantGoals {
		t.Fatalf("expected %d listed goals, got:\n%s", maxRelevantGoals, out)
	}
}

func TestKeywords_DropsStopWordsAndShortTokens(t *testing.T) {
	kw := keywords("I want to run today!")
	if kw["want"] {
		t.Errorf("stop word 'want' should be dropped")
	}
	if kw["to"] {
		t.Errorf("stop word 'to' should be dropped")
	}
	if !kw["run"] {
		t.Errorf("keyword 'run' should be present")
	}
	if !kw["today"] {
		t.Errorf("keyword 'today' should be present")
	}
}
