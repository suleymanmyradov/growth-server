package prompts

import (
	"fmt"
	"sort"
	"strings"

	aiprompts "github.com/suleymanmyradov/growth-server/pkg/ai/prompts"
	"github.com/suleymanmyradov/growth-server/pkg/ai/tokens"
)

// DefaultContextBudgetTokens is the token budget for the personalization
// sections that surround the user message. It is sized so that, together with
// the system prompt and a typical conversation history, the full request stays
// well under common model context windows. The assembler fills this budget by
// priority and drops the lowest-priority sections first when it is tight.
const DefaultContextBudgetTokens = 1200

// Per-field caps on how many goals/habits are listed individually before the
// remainder is summarized as a count. These are defense-in-depth on top of the
// token budget; the budget is the primary control.
const (
	maxRelevantGoals  = 5
	maxRelevantHabits = 5
)

// SectionBreakdown records the token cost of each context section and whether
// it made it into the final prompt. It powers the per-section observability
// (see metrics in the coaching logic) so the budget can be tuned with data.
type SectionBreakdown struct {
	Section  string
	Tokens   int
	Included bool
}

// section is a priority-tiered context block. Lower priority sections are
// included first and dropped last when the budget is exhausted.
type section struct {
	priority int
	name     string
	render   func() string
}

// AssembleContextWithBreakdown builds the user prompt (the user message, always
// included, followed by priority-tiered context sections filled within
// budgetTokens) and returns a per-section token breakdown for observability.
// The user message itself is mandatory and is not counted against the budget.
func AssembleContextWithBreakdown(in PersonalizedCoachingInput, budgetTokens int) (string, []SectionBreakdown) {
	if budgetTokens <= 0 {
		budgetTokens = DefaultContextBudgetTokens
	}

	var b strings.Builder

	// The user message is the actual question; it is always emitted and is not
	// subject to the context budget.
	userMsg := aiprompts.SanitizeAndTruncate(in.UserMessage, aiprompts.MaxFieldUserMessage)
	fmt.Fprintf(&b, "User's Message: %s\n\n", userMsg)

	breakdown := fillSections(&b, buildSections(in), budgetTokens)
	return b.String(), breakdown
}

// fillSections appends sections to b in priority order until the token budget
// is exhausted, recording a breakdown. Sections whose render returns "" are
// skipped entirely (not counted as dropped).
func fillSections(b *strings.Builder, sections []section, budget int) []SectionBreakdown {
	sort.SliceStable(sections, func(i, j int) bool {
		return sections[i].priority < sections[j].priority
	})

	breakdown := make([]SectionBreakdown, 0, len(sections))
	used := 0
	for _, s := range sections {
		text := s.render()
		if text == "" {
			continue
		}
		n := tokens.Count(text)
		included := used+n <= budget
		if included {
			b.WriteString(text)
			used += n
		}
		breakdown = append(breakdown, SectionBreakdown{
			Section:  s.name,
			Tokens:   n,
			Included: included,
		})
	}
	return breakdown
}

func buildSections(in PersonalizedCoachingInput) []section {
	return []section{
		{0, "user_profile", func() string { return renderUserProfile(in) }},
		{0, "coaching_profile", func() string { return renderCoachingProfile(in) }},
		{1, "active_goals", func() string { return renderRelevantGoals(in) }},
		{1, "active_habits", func() string { return renderRelevantHabits(in) }},
		{2, "pattern_insights", func() string { return renderPatternInsights(in) }},
		{3, "check_in_digest", func() string { return renderCheckInDigest(in) }},
		{4, "common_blockers", func() string { return renderCommonBlockers(in) }},
		// Lowest priority: long-term memory retrieval. Dropped first when the
		// budget is tight, so retrieval can never push the prompt over budget.
		{5, "relevant_memories", func() string { return renderRelevantMemories(in) }},
	}
}

// maxMemorySnippets caps how many retrieved snippets are ever rendered, as
// defense-in-depth on top of the token budget and the retriever's own limit.
const maxMemorySnippets = 5

// maxMemorySnippetChars caps each rendered snippet so a single verbose memory
// cannot dominate the section.
const maxMemorySnippetChars = 240

// renderRelevantMemories renders the bounded "Relevant past context" section.
// Every snippet is wrapped via WrapUserContent (prompt-injection defense) and
// SanitizeAndTruncate so retrieved free-text is treated as untrusted data.
func renderRelevantMemories(in PersonalizedCoachingInput) string {
	if len(in.RelevantMemories) == 0 {
		return ""
	}
	n := len(in.RelevantMemories)
	if n > maxMemorySnippets {
		n = maxMemorySnippets
	}
	var b strings.Builder
	b.WriteString("\n--- Relevant Past Context ---\n")
	for i := 0; i < n; i++ {
		m := in.RelevantMemories[i]
		body := aiprompts.SanitizeAndTruncate(m.Content, maxMemorySnippetChars)
		if body == "" {
			continue
		}
		label := memoryLabel(m)
		if m.CreatedAt.IsZero() {
			b.WriteString(aiprompts.WrapUserContent(label, body))
		} else {
			b.WriteString(aiprompts.WrapUserContent(label+" "+m.CreatedAt.Format("2006-01-02"), body))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// memoryLabel maps an entity type to a human-readable source tag.
func memoryLabel(m MemorySnippet) string {
	switch m.EntityType {
	case "check_in":
		if m.HabitName != "" {
			return "past check-in note for " + m.HabitName
		}
		return "past check-in note"
	case "conversation_message":
		if m.Role == "assistant" {
			return "past assistant message"
		}
		return "past user message"
	case "weekly_review":
		return "past weekly review summary"
	default:
		return "past note"
	}
}

func renderUserProfile(in PersonalizedCoachingInput) string {
	if in.UserFullName == "" && in.UserBio == "" && in.UserLocation == "" && len(in.UserInterests) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("--- User Profile ---\n")
	if in.UserFullName != "" {
		fmt.Fprintf(&b, "Name: %s\n", in.UserFullName)
	}
	if in.UserBio != "" {
		fmt.Fprintf(&b, "Bio: %s\n", in.UserBio)
	}
	if in.UserLocation != "" {
		fmt.Fprintf(&b, "Location: %s\n", in.UserLocation)
	}
	if len(in.UserInterests) > 0 {
		fmt.Fprintf(&b, "Interests: %s\n", strings.Join(in.UserInterests, ", "))
	}
	b.WriteString("\n")
	return b.String()
}

func renderCoachingProfile(in PersonalizedCoachingInput) string {
	var b strings.Builder
	b.WriteString("--- Coaching Profile ---\n")
	fmt.Fprintf(&b, "Accountability Style: %s\n", in.AccountabilityStyle)
	fmt.Fprintf(&b, "Preferred Tone: %s\n", in.PreferredTone)
	fmt.Fprintf(&b, "Difficulty Preference: %s\n", in.DifficultyPreference)
	return b.String()
}

// renderRelevantGoals lists the goals most relevant to the user's message
// (lexical keyword overlap), capped at maxRelevantGoals, and summarizes the
// rest as a count so the section stays near-constant size.
func renderRelevantGoals(in PersonalizedCoachingInput) string {
	selected, rest := selectRelevant(in.UserMessage, in.ActiveGoals, maxRelevantGoals)
	if len(selected) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n--- Active Goals ---\n")
	for i, g := range selected {
		fmt.Fprintf(&b, "%d. %s\n", i+1, aiprompts.SanitizeAndTruncate(g, aiprompts.MaxFieldGoal))
	}
	if rest > 0 {
		fmt.Fprintf(&b, "(plus %d other active goals)\n", rest)
	}
	return b.String()
}

func renderRelevantHabits(in PersonalizedCoachingInput) string {
	selected, rest := selectRelevant(in.UserMessage, in.ActiveHabits, maxRelevantHabits)
	if len(selected) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n--- Active Habits ---\n")
	for i, h := range selected {
		fmt.Fprintf(&b, "%d. %s\n", i+1, aiprompts.SanitizeAndTruncate(h, aiprompts.MaxFieldHabit))
	}
	if rest > 0 {
		fmt.Fprintf(&b, "(plus %d other active habits)\n", rest)
	}
	return b.String()
}

// renderPatternInsights emits the compact, pre-aggregated pattern insights.
// These are aggregates (not raw check-in rows) and are the primary check-in
// representation; raw check-ins are never enumerated in the prompt.
func renderPatternInsights(in PersonalizedCoachingInput) string {
	if len(in.PatternInsights) == 0 {
		return ""
	}
	keys := make([]string, 0, len(in.PatternInsights))
	for k := range in.PatternInsights {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("\n--- Pattern Insights ---\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %s\n", k, aiprompts.SanitizeAndTruncate(in.PatternInsights[k], aiprompts.MaxFieldNote))
	}
	return b.String()
}

// renderCheckInDigest emits the pre-computed check-in digest (an aggregate, not
// raw rows). The digest is assembled upstream from check-in counts and trend.
func renderCheckInDigest(in PersonalizedCoachingInput) string {
	if in.RecentCheckInsSummary == "" {
		return ""
	}
	return fmt.Sprintf("\n--- Recent Check-in Summary ---\n%s\n",
		aiprompts.SanitizeAndTruncate(in.RecentCheckInsSummary, aiprompts.MaxFieldNote))
}

func renderCommonBlockers(in PersonalizedCoachingInput) string {
	if len(in.CommonBlockers) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n--- Common Blockers ---\n")
	for i, bl := range in.CommonBlockers {
		fmt.Fprintf(&b, "%d. %s\n", i+1, aiprompts.SanitizeAndTruncate(bl, aiprompts.MaxFieldBlocker))
	}
	return b.String()
}

// selectRelevant returns the top-k items most relevant to the user message,
// scored by lexical keyword overlap, plus the count of items not selected.
// Ties preserve the original order (stable sort). When nothing matches (e.g.
// a casual greeting), the first k items are returned as background context.
func selectRelevant(userMsg string, items []string, k int) ([]string, int) {
	if len(items) == 0 {
		return nil, 0
	}
	if k <= 0 || k > len(items) {
		k = len(items)
	}
	kw := keywords(userMsg)

	type scored struct {
		idx, score int
	}
	sc := make([]scored, len(items))
	for i, it := range items {
		sc[i] = scored{i, relevance(it, kw)}
	}
	sort.SliceStable(sc, func(i, j int) bool { return sc[i].score > sc[j].score })

	selected := make([]string, k)
	for i := 0; i < k; i++ {
		selected[i] = items[sc[i].idx]
	}
	return selected, len(items) - k
}

// relevance scores an item by how many of its keywords appear in the user
// message. Higher means more topically relevant to what the user just said.
func relevance(item string, kw map[string]bool) int {
	score := 0
	for w := range keywords(item) {
		if kw[w] {
			score++
		}
	}
	return score
}

// keywords extracts a lowercase keyword set from s, dropping short tokens and
// common stop words. Good enough for lexical relevance scoring without
// embeddings.
func keywords(s string) map[string]bool {
	m := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,!?;:\"'()[]{}-–—/")
		if len(w) < 3 || stopWords[w] {
			continue
		}
		m[w] = true
	}
	return m
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"i": true, "im": true, "my": true, "me": true, "to": true, "of": true,
	"in": true, "on": true, "for": true, "with": true, "is": true, "am": true,
	"are": true, "was": true, "were": true, "be": true, "been": true,
	"being": true, "have": true, "has": true, "had": true, "do": true,
	"does": true, "did": true, "will": true, "would": true, "could": true,
	"should": true, "this": true, "that": true, "it": true, "at": true,
	"by": true, "from": true, "as": true, "so": true, "if": true, "not": true,
	"no": true, "yes": true, "just": true, "really": true, "very": true,
	"about": true, "what": true, "how": true, "why": true, "when": true,
	"who": true, "can": true, "cant": true, "get": true, "got": true,
	"want": true, "need": true, "help": true, "some": true, "any": true,
	"all": true, "its": true,
}
