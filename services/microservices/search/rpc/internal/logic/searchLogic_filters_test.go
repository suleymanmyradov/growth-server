package logic

import (
	"strings"
	"testing"

	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"
)

const testUserID = "8f14e45f-ea0c-4f9b-9a1e-2b3c4d5e6f70"

// The security clause must always be present and first: Meilisearch ANDs the
// elements of the filter array, so element 0 gates every other clause.
func TestBuildFiltersSecurityClauseAlwaysFirst(t *testing.T) {
	tests := []struct {
		name string
		req  *search.SearchRequest
		want string
	}{
		{
			name: "anonymous sees public only",
			req:  &search.SearchRequest{},
			want: `visibility = "public"`,
		},
		{
			name: "authenticated sees public or own",
			req:  &search.SearchRequest{UserId: testUserID},
			want: `(visibility = "public" OR user_id = "` + testUserID + `")`,
		},
		{
			name: "clause survives other filters",
			req: &search.SearchRequest{
				UserId:   testUserID,
				Types:    []string{"goal"},
				Category: "mindset",
				Status:   "published",
			},
			want: `(visibility = "public" OR user_id = "` + testUserID + `")`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filters, err := buildFilters(tc.req)
			if err != nil {
				t.Fatalf("buildFilters: %v", err)
			}
			if len(filters) == 0 {
				t.Fatal("no filters returned; the security clause is mandatory")
			}
			if filters[0] != tc.want {
				t.Errorf("security clause = %q, want %q", filters[0], tc.want)
			}
		})
	}
}

// A malformed user id reaches the filter expression, so it must be rejected
// rather than degraded to a public-only search.
func TestBuildFiltersRejectsNonUUIDUserID(t *testing.T) {
	for _, userID := range []string{
		"not-a-uuid",
		`x" OR user_id = "` + testUserID,
		testUserID + `" OR visibility = "private`,
		`"`,
	} {
		if _, err := buildFilters(&search.SearchRequest{UserId: userID}); err == nil {
			t.Errorf("userID %q: expected error, got nil", userID)
		}
	}
}

// Closed sets are rejected outright rather than silently matching nothing —
// a wrong value is a caller bug, and a silent empty result set hides it.
// (This is how search_articles shipped with Types=["articles"] and returned
// no articles at all.)
func TestBuildFiltersRejectsUnknownClosedSetValues(t *testing.T) {
	t.Run("type", func(t *testing.T) {
		for _, typ := range []string{"articles", "Article", "conversation_message", `article" OR visibility = "private`} {
			if _, err := buildFilters(&search.SearchRequest{Types: []string{typ}}); err == nil {
				t.Errorf("type %q: expected error, got nil", typ)
			}
		}
	})

	t.Run("status", func(t *testing.T) {
		for _, st := range []string{"active", "Published", `published" OR visibility = "private`} {
			if _, err := buildFilters(&search.SearchRequest{Status: st}); err == nil {
				t.Errorf("status %q: expected error, got nil", st)
			}
		}
	})

	t.Run("accepts real values", func(t *testing.T) {
		for _, typ := range []string{"article", "goal", "habit"} {
			if _, err := buildFilters(&search.SearchRequest{Types: []string{typ}}); err != nil {
				t.Errorf("type %q should be accepted: %v", typ, err)
			}
		}
		for _, st := range []string{"draft", "published"} {
			if _, err := buildFilters(&search.SearchRequest{Status: st}); err != nil {
				t.Errorf("status %q should be accepted: %v", st, err)
			}
		}
	})
}

// Category is an open set (domain-defined), so it is escaped rather than
// allowlisted. The escaped value must not be able to terminate its own
// string literal and inject filter syntax.
func TestBuildFiltersEscapesCategory(t *testing.T) {
	tests := []struct {
		name     string
		category string
	}{
		{"quote breakout", `x" OR visibility = "private`},
		{"backslash", `x\`},
		{"backslash then quote", `x\" OR user_id = "y`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filters, err := buildFilters(&search.SearchRequest{Category: tc.category})
			if err != nil {
				t.Fatalf("buildFilters: %v", err)
			}
			var clause string
			for _, f := range filters {
				if strings.HasPrefix(f, "category = ") {
					clause = f
				}
			}
			if clause == "" {
				t.Fatal("no category clause produced")
			}
			// Every quote inside the literal must be backslash-escaped, so the
			// clause cannot contain a bare `"` that ends the string early.
			inner := strings.TrimSuffix(strings.TrimPrefix(clause, `category = "`), `"`)
			for i := 0; i < len(inner); i++ {
				if inner[i] == '\\' {
					i++ // skip the escaped character
					continue
				}
				if inner[i] == '"' {
					t.Errorf("unescaped quote at %d in %q", i, clause)
				}
			}
		})
	}
}

func TestQuoteFilterValue(t *testing.T) {
	tests := []struct{ in, want string }{
		{`plain`, `"plain"`},
		{`with space`, `"with space"`},
		{`quote"here`, `"quote\"here"`},
		{`back\slash`, `"back\\slash"`},
		{`both\"x`, `"both\\\"x"`},
	}
	for _, tc := range tests {
		if got := quoteFilterValue(tc.in); got != tc.want {
			t.Errorf("quoteFilterValue(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
