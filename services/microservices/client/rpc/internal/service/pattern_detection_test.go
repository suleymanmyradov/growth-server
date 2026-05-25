package service

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

func TestAnalyzeFullFromData_EmptyInputs(t *testing.T) {
	svc := &PatternDetectionService{}

	insights := svc.AnalyzeFullFromData([]db.CheckIn{}, []db.Habit{}, time.UTC)

	if insights == nil {
		t.Fatal("Expected non-nil insights")
	}

	// Should return zero-value struct with no panics
	// Empty inputs result in "no_data" for completion pattern
	if insights.CompletionPattern != "no_data" {
		t.Errorf("Expected 'no_data' completion pattern, got %s", insights.CompletionPattern)
	}
	if insights.TopBlocker != "" {
		t.Errorf("Expected empty top blocker, got %s", insights.TopBlocker)
	}
	if len(insights.RiskFactors) != 0 {
		t.Errorf("Expected no risk factors, got %d", len(insights.RiskFactors))
	}
}

func TestAnalyzeFullFromData_TimePatterns(t *testing.T) {
	svc := &PatternDetectionService{}

	now := time.Now()

	// Test edge case: hour 0 (midnight)
	checkIns := []db.CheckIn{
		{
			ID:        uuid.New(),
			CreatedAt: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
			Status:    "completed",
			Mood:      sql.NullString{String: "great", Valid: true},
		},
	}

	insights := svc.AnalyzeFullFromData(checkIns, []db.Habit{}, time.UTC)

	// Should not panic and should return a time range
	if insights.BestTimeOfDay == "" {
		t.Error("Expected best time of day to be set")
	}
}

func TestAnalyzeFullFromData_MoodEnergyCorrelation(t *testing.T) {
	svc := &PatternDetectionService{}

	now := time.Now()

	// Test with only high mood inputs
	checkIns := []db.CheckIn{
		{
			ID:        uuid.New(),
			CreatedAt: now,
			Status:    "completed",
			Mood:      sql.NullString{String: "great", Valid: true},
		},
		{
			ID:        uuid.New(),
			CreatedAt: now.Add(time.Hour),
			Status:    "completed",
			Mood:      sql.NullString{String: "good", Valid: true},
		},
	}

	insights := svc.AnalyzeFullFromData(checkIns, []db.Habit{}, time.UTC)

	// Should not panic
	if insights.MoodEnergyCorrelation == "" {
		t.Error("Expected mood energy correlation to be set")
	}
}

func TestToFlatMap_BackwardCompatibility(t *testing.T) {
	insights := &PatternInsights{
		CompletionPattern:     "high_consistency",
		TopBlocker:            "tired",
		BestTimeOfDay:         "morning (9am-12pm)",
		HardestTimeOfDay:      "night (9pm-5am)",
		MoodEnergyCorrelation: "positive_mood_correlation",
		StreakPattern:         "strong_momentum",
		RiskFactors:           []string{"mood_dependency", "weekend_slump"}, // sorted
	}

	flatMap := insights.ToFlatMap()

	// Check backward compatibility keys exist
	if flatMap["completion_pattern"] != "high_consistency" {
		t.Errorf("Expected completion_pattern to be high_consistency, got %s", flatMap["completion_pattern"])
	}
	if flatMap["top_blocker"] != "tired" {
		t.Errorf("Expected top_blocker to be tired, got %s", flatMap["top_blocker"])
	}

	// Check new keys
	if flatMap["best_time_of_day"] != "morning (9am-12pm)" {
		t.Errorf("Expected best_time_of_day to be morning (9am-12pm), got %s", flatMap["best_time_of_day"])
	}
	if flatMap["hardest_time_of_day"] != "night (9pm-5am)" {
		t.Errorf("Expected hardest_time_of_day to be night (9pm-5am), got %s", flatMap["hardest_time_of_day"])
	}
	if flatMap["mood_energy_correlation"] != "positive_mood_correlation" {
		t.Errorf("Expected mood_energy_correlation to be positive_mood_correlation, got %s", flatMap["mood_energy_correlation"])
	}
	if flatMap["streak_pattern"] != "strong_momentum" {
		t.Errorf("Expected streak_pattern to be strong_momentum, got %s", flatMap["streak_pattern"])
	}
	if flatMap["risk_factors"] != "mood_dependency,weekend_slump" {
		t.Errorf("Expected risk_factors to be mood_dependency,weekend_slump (sorted), got %s", flatMap["risk_factors"])
	}
}

func TestToFlatMap_NoData(t *testing.T) {
	insights := &PatternInsights{
		CompletionPattern:     "no_data",
		MoodEnergyCorrelation: "no_data",
		StreakPattern:         "no_data",
	}

	flatMap := insights.ToFlatMap()

	// no_data values should not be included in the map
	if flatMap["mood_energy_correlation"] != "" {
		t.Errorf("Expected mood_energy_correlation to be empty for no_data, got %s", flatMap["mood_energy_correlation"])
	}
	if flatMap["streak_pattern"] != "" {
		t.Errorf("Expected streak_pattern to be empty for no_data, got %s", flatMap["streak_pattern"])
	}
}

func TestAnalyzeLite(t *testing.T) {
	svc := &PatternDetectionService{}

	now := time.Now()
	checkIns := []db.CheckIn{
		{
			ID:        uuid.New(),
			CreatedAt: now,
			Status:    "completed",
		},
	}
	habits := []db.Habit{
		{
			ID:   uuid.New(),
			Name: "Exercise",
		},
	}

	flatMap := svc.AnalyzeLite(checkIns, habits, time.UTC)

	// Should return a map with the expected keys
	if flatMap == nil {
		t.Fatal("Expected non-nil map")
	}

	// Should have completion_pattern since we have check-ins
	if flatMap["completion_pattern"] == "" {
		t.Error("Expected completion_pattern to be set")
	}
}

func TestAnalyzeFullFromData_TimezoneAware(t *testing.T) {
	svc := &PatternDetectionService{}
	
	// Create a check-in at 10 PM UTC (which would be 5 PM in EST)
	utcTime := time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC)
	
	checkIns := []db.CheckIn{
		{
			ID:        uuid.New(),
			CreatedAt: utcTime,
			Status:    "completed",
		},
	}
	
	// Test with UTC timezone
	utcInsights := svc.AnalyzeFullFromData(checkIns, []db.Habit{}, time.UTC)
	if utcInsights.BestTimeOfDay == "" {
		t.Error("Expected best time of day to be set for UTC")
	}
	
	// Test with EST timezone (UTC-5)
	estLoc, _ := time.LoadLocation("America/New_York")
	estInsights := svc.AnalyzeFullFromData(checkIns, []db.Habit{}, estLoc)
	if estInsights.BestTimeOfDay == "" {
		t.Error("Expected best time of day to be set for EST")
	}
	
	// The time should be different between UTC and EST
	// 10 PM UTC = 5 PM EST (evening vs afternoon)
	if utcInsights.BestTimeOfDay == estInsights.BestTimeOfDay {
		t.Logf("Warning: Timezone conversion may not be working as expected. UTC: %s, EST: %s", 
			utcInsights.BestTimeOfDay, estInsights.BestTimeOfDay)
	}
}

func TestIdentifyRiskFactors_CappingAndSorting(t *testing.T) {
	svc := &PatternDetectionService{}
	
	now := time.Now()
	
	// Create check-ins that would trigger many risk factors
	checkIns := []db.CheckIn{
		{
			ID:        uuid.New(),
			CreatedAt: now,
			Status:    "missed",
		},
		{
			ID:        uuid.New(),
			CreatedAt: now.Add(time.Hour),
			Status:    "missed",
		},
	}
	
	insights := &PatternInsights{
		CompletionPattern:     "low_consistency",
		TopBlocker:            "tired",
		MoodEnergyCorrelation: "negative_mood_correlation",
		StreakPattern:         "starting_fresh",
	}
	
	risks := svc.identifyRiskFactors(checkIns, insights, time.UTC)
	
	// Should cap at 5 risk factors
	if len(risks) > 5 {
		t.Errorf("Expected max 5 risk factors, got %d", len(risks))
	}
	
	// Should be sorted alphabetically
	for i := 0; i < len(risks)-1; i++ {
		if risks[i] > risks[i+1] {
			t.Errorf("Risk factors not sorted: %s comes before %s", risks[i], risks[i+1])
		}
	}
}

func TestTimezoneRegression_Ashgabat(t *testing.T) {
	svc := &PatternDetectionService{}
	
	// Asia/Ashgabat is UTC+5
	// Test with a time that clearly falls into morning in Ashgabat
	// 2025-01-01T04:00:00Z = 2025-01-01T09:00:00 in Ashgabat (9am exactly)
	// This should bucket into "morning (9am-12pm)"
	utcTime := time.Date(2025, 1, 1, 4, 0, 0, 0, time.UTC)
	
	checkIns := []db.CheckIn{
		{
			ID:        uuid.New(),
			CreatedAt: utcTime,
			Status:    "completed",
		},
	}
	
	ashgabatLoc, err := time.LoadLocation("Asia/Ashgabat")
	if err != nil {
		t.Fatalf("Failed to load Asia/Ashgabat timezone: %v", err)
	}
	
	insights := svc.AnalyzeFullFromData(checkIns, []db.Habit{}, ashgabatLoc)
	
	// 04:00 UTC = 09:00 Ashgabat, which should be "morning (9am-12pm)"
	if insights.BestTimeOfDay != "morning (9am-12pm)" {
		t.Errorf("Expected 'morning (9am-12pm)' for 09:00 local time, got '%s'", insights.BestTimeOfDay)
	}
	
	// Now test with UTC to ensure it would give a different result
	utcInsights := svc.AnalyzeFullFromData(checkIns, []db.Habit{}, time.UTC)
	
	// 04:00 UTC = 04:00 UTC, which should be "night (9pm-5am)" 
	if utcInsights.BestTimeOfDay != "night (9pm-5am)" {
		t.Errorf("Expected 'night (9pm-5am)' for 04:00 UTC, got '%s'", utcInsights.BestTimeOfDay)
	}
	
	// The key assertion: timezone conversion should produce different results
	if insights.BestTimeOfDay == utcInsights.BestTimeOfDay {
		t.Errorf("Timezone conversion not working - both UTC and Ashgabat produced same result: '%s'", insights.BestTimeOfDay)
	}
}
