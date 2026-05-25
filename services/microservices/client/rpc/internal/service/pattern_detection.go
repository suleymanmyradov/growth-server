package service

import (
	"strings"
	"time"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

// PatternDetectionService analyzes user behavior patterns to provide insights
type PatternDetectionService struct {
	repo *repository.Repository
}

func NewPatternDetectionService(repo *repository.Repository) *PatternDetectionService {
	return &PatternDetectionService{
		repo: repo,
	}
}

// PatternInsights contains detected patterns from user behavior
type PatternInsights struct {
	CompletionPattern     string            `json:"completion_pattern"`
	TopBlocker            string            `json:"top_blocker"`
	BlockerFrequency      map[string]int    `json:"blocker_frequency"`
	BestTimeOfDay         string            `json:"best_time_of_day"`
	HardestTimeOfDay      string            `json:"hardest_time_of_day"`
	MoodEnergyCorrelation string            `json:"mood_energy_correlation"`
	StreakPattern         string            `json:"streak_pattern"`
	HabitDifficulty       map[string]string `json:"habit_difficulty"`
	RiskFactors           []string          `json:"risk_factors"`
}

// AnalyzeFullFromData skips the DB and analyzes already-loaded data.
func (s *PatternDetectionService) AnalyzeFullFromData(checkIns []db.CheckIn, habits []db.Habit, userLoc *time.Location) *PatternInsights {
	insights := &PatternInsights{
		BlockerFrequency: make(map[string]int),
		HabitDifficulty:  make(map[string]string),
		RiskFactors:      []string{},
	}

	// Analyze completion patterns
	insights.CompletionPattern = s.analyzeCompletionPattern(checkIns)

	// Analyze blockers
	insights.TopBlocker, insights.BlockerFrequency = s.analyzeBlockers(checkIns)

	// Analyze time patterns
	insights.BestTimeOfDay, insights.HardestTimeOfDay = s.analyzeTimePatterns(checkIns, userLoc)

	// Analyze mood/energy correlation
	insights.MoodEnergyCorrelation = s.analyzeMoodEnergyCorrelation(checkIns)

	// Analyze streak patterns
	insights.StreakPattern = s.analyzeStreakPatterns(habits)

	// Analyze habit difficulty
	insights.HabitDifficulty = s.analyzeHabitDifficulty(checkIns, habits)

	// Identify risk factors
	insights.RiskFactors = s.identifyRiskFactors(checkIns, insights, userLoc)

	return insights
}

// AnalyzeLite returns the flat map[string]string used by the proto.
// Internally calls AnalyzeFullFromData and projects down.
func (s *PatternDetectionService) AnalyzeLite(checkIns []db.CheckIn, habits []db.Habit, userLoc *time.Location) map[string]string {
	insights := s.AnalyzeFullFromData(checkIns, habits, userLoc)
	return insights.ToFlatMap()
}

// ToFlatMap projects the rich PatternInsights struct to a flat map for proto compatibility.
func (p *PatternInsights) ToFlatMap() map[string]string {
	m := map[string]string{}
	if p.CompletionPattern != "" {
		m["completion_pattern"] = p.CompletionPattern
	}
	if p.TopBlocker != "" {
		m["top_blocker"] = p.TopBlocker
	}
	if p.BestTimeOfDay != "" {
		m["best_time_of_day"] = p.BestTimeOfDay
	}
	if p.HardestTimeOfDay != "" {
		m["hardest_time_of_day"] = p.HardestTimeOfDay
	}
	if p.MoodEnergyCorrelation != "" && p.MoodEnergyCorrelation != "no_data" {
		m["mood_energy_correlation"] = p.MoodEnergyCorrelation
	}
	if p.StreakPattern != "" && p.StreakPattern != "no_data" {
		m["streak_pattern"] = p.StreakPattern
	}
	if len(p.RiskFactors) > 0 {
		// RiskFactors are already sorted in identifyRiskFactors, but ensure here too
		riskFactorsCopy := make([]string, len(p.RiskFactors))
		copy(riskFactorsCopy, p.RiskFactors)
		for i := 0; i < len(riskFactorsCopy); i++ {
			for j := i + 1; j < len(riskFactorsCopy); j++ {
				if riskFactorsCopy[i] > riskFactorsCopy[j] {
					riskFactorsCopy[i], riskFactorsCopy[j] = riskFactorsCopy[j], riskFactorsCopy[i]
				}
			}
		}
		m["risk_factors"] = strings.Join(riskFactorsCopy, ",")
	}
	return m
}

// analyzeCompletionPattern determines the user's consistency level
func (s *PatternDetectionService) analyzeCompletionPattern(checkIns []db.CheckIn) string {
	if len(checkIns) == 0 {
		return "no_data"
	}

	completed := 0
	for _, checkIn := range checkIns {
		if checkIn.Status == "completed" {
			completed++
		}
	}

	rate := float64(completed) / float64(len(checkIns)) * 100

	switch {
	case rate >= 80:
		return "high_consistency"
	case rate >= 60:
		return "moderate_consistency"
	case rate >= 40:
		return "inconsistent"
	default:
		return "low_consistency"
	}
}

// analyzeBlockers identifies the most common blockers
func (s *PatternDetectionService) analyzeBlockers(checkIns []db.CheckIn) (string, map[string]int) {
	blockerCounts := make(map[string]int)

	for _, checkIn := range checkIns {
		if checkIn.Blocker.Valid && checkIn.Blocker.String != "" {
			blockerCounts[checkIn.Blocker.String]++
		}
	}

	if len(blockerCounts) == 0 {
		return "", blockerCounts
	}

	topBlocker := ""
	maxCount := 0
	for blocker, count := range blockerCounts {
		if count > maxCount {
			maxCount = count
			topBlocker = blocker
		}
	}

	return topBlocker, blockerCounts
}

// analyzeTimePatterns identifies best and worst times for habit completion
func (s *PatternDetectionService) analyzeTimePatterns(checkIns []db.CheckIn, userLoc *time.Location) (string, string) {
	if len(checkIns) == 0 {
		return "", ""
	}

	hourlyCompletion := make(map[int]int) // hour -> completed count
	hourlyTotal := make(map[int]int)      // hour -> total count

	for _, checkIn := range checkIns {
		localTime := checkIn.CreatedAt.In(userLoc)
		hour := localTime.Hour()
		hourlyTotal[hour]++
		if checkIn.Status == "completed" {
			hourlyCompletion[hour]++
		}
	}

	if len(hourlyTotal) == 0 {
		return "", ""
	}

	// Find best and worst hours
	bestHour := -1
	bestRate := 0.0
	worstHour := -1
	worstRate := 1.0

	for hour, total := range hourlyTotal {
		if total == 0 {
			continue
		}
		rate := float64(hourlyCompletion[hour]) / float64(total)

		if rate > bestRate {
			bestRate = rate
			bestHour = hour
		}
		if rate < worstRate {
			worstRate = rate
			worstHour = hour
		}
	}

	bestTime := ""
	worstTime := ""

	if bestHour >= 0 {
		bestTime = formatTimeRange(bestHour)
	}
	if worstHour >= 0 {
		worstTime = formatTimeRange(worstHour)
	}

	return bestTime, worstTime
}

// analyzeMoodEnergyCorrelation analyzes relationship between mood/energy and completion
func (s *PatternDetectionService) analyzeMoodEnergyCorrelation(checkIns []db.CheckIn) string {
	if len(checkIns) == 0 {
		return "no_data"
	}

	highMoodCompleted := 0
	highMoodTotal := 0
	lowMoodCompleted := 0
	lowMoodTotal := 0

	for _, checkIn := range checkIns {
		if !checkIn.Mood.Valid {
			continue
		}

		isHighMood := checkIn.Mood.String == "great" || checkIn.Mood.String == "good"
		isLowMood := checkIn.Mood.String == "bad" || checkIn.Mood.String == "terrible"

		if isHighMood {
			highMoodTotal++
			if checkIn.Status == "completed" {
				highMoodCompleted++
			}
		} else if isLowMood {
			lowMoodTotal++
			if checkIn.Status == "completed" {
				lowMoodCompleted++
			}
		}
	}

	if highMoodTotal == 0 && lowMoodTotal == 0 {
		return "no_data"
	}

	highRate := 0.0
	if highMoodTotal > 0 {
		highRate = float64(highMoodCompleted) / float64(highMoodTotal)
	}

	lowRate := 0.0
	if lowMoodTotal > 0 {
		lowRate = float64(lowMoodCompleted) / float64(lowMoodTotal)
	}

	if highRate > lowRate+0.2 {
		return "positive_mood_correlation"
	} else if lowRate > highRate+0.2 {
		return "negative_mood_correlation"
	} else {
		return "no_strong_correlation"
	}
}

// analyzeStreakPatterns analyzes streak behavior across habits
func (s *PatternDetectionService) analyzeStreakPatterns(habits []db.Habit) string {
	if len(habits) == 0 {
		return "no_data"
	}

	longStreaks := 0
	shortStreaks := 0
	noStreaks := 0

	for _, habit := range habits {
		if habit.Streak.Valid {
			if habit.Streak.Int32 >= 7 {
				longStreaks++
			} else if habit.Streak.Int32 >= 3 {
				shortStreaks++
			} else {
				noStreaks++
			}
		} else {
			noStreaks++
		}
	}

	if longStreaks > len(habits)/2 {
		return "strong_momentum"
	} else if shortStreaks > len(habits)/2 {
		return "building_momentum"
	} else if noStreaks > len(habits)/2 {
		return "starting_fresh"
	} else {
		return "mixed_streaks"
	}
}

// analyzeHabitDifficulty determines which habits are easy vs difficult for the user
func (s *PatternDetectionService) analyzeHabitDifficulty(checkIns []db.CheckIn, habits []db.Habit) map[string]string {
	habitStats := make(map[string]int)  // habitID -> completed count
	habitTotals := make(map[string]int) // habitID -> total count

	for _, checkIn := range checkIns {
		habitID := checkIn.HabitID.String()
		habitTotals[habitID]++
		if checkIn.Status == "completed" {
			habitStats[habitID]++
		}
	}

	difficulty := make(map[string]string)
	for _, habit := range habits {
		habitID := habit.ID.String()
		total := habitTotals[habitID]
		if total == 0 {
			difficulty[habit.Name] = "unknown"
			continue
		}

		completed := habitStats[habitID]
		rate := float64(completed) / float64(total)

		switch {
		case rate >= 0.8:
			difficulty[habit.Name] = "easy"
		case rate >= 0.5:
			difficulty[habit.Name] = "moderate"
		default:
			difficulty[habit.Name] = "challenging"
		}
	}

	return difficulty
}

// identifyRiskFactors finds potential risks to habit consistency
func (s *PatternDetectionService) identifyRiskFactors(checkIns []db.CheckIn, insights *PatternInsights, userLoc *time.Location) []string {
	risks := []string{}

	if insights.CompletionPattern == "low_consistency" {
		risks = append(risks, "inconsistency_pattern")
	}

	if insights.TopBlocker != "" {
		risks = append(risks, "recurring_blocker")
	}

	if insights.MoodEnergyCorrelation == "negative_mood_correlation" {
		risks = append(risks, "mood_dependency")
	}

	if insights.StreakPattern == "starting_fresh" {
		risks = append(risks, "low_momentum")
	}

	// Check for weekend vs weekday patterns
	weekdayCompleted := 0
	weekdayTotal := 0
	weekendCompleted := 0
	weekendTotal := 0

	for _, checkIn := range checkIns {
		localTime := checkIn.CreatedAt.In(userLoc)
		day := localTime.Weekday()
		isWeekend := day == time.Saturday || day == time.Sunday

		if isWeekend {
			weekendTotal++
			if checkIn.Status == "completed" {
				weekendCompleted++
			}
		} else {
			weekdayTotal++
			if checkIn.Status == "completed" {
				weekdayCompleted++
			}
		}
	}

	if weekdayTotal > 0 && weekendTotal > 0 {
		weekdayRate := float64(weekdayCompleted) / float64(weekdayTotal)
		weekendRate := float64(weekendCompleted) / float64(weekendTotal)

		if weekdayRate > weekendRate+0.3 {
			risks = append(risks, "weekend_slump")
		} else if weekendRate > weekdayRate+0.3 {
			risks = append(risks, "weekday_struggle")
		}
	}

	// Cap risk factors to top 5 and sort alphabetically for consistency
	if len(risks) > 5 {
		risks = risks[:5]
	}
	// Sort alphabetically
	for i := 0; i < len(risks); i++ {
		for j := i + 1; j < len(risks); j++ {
			if risks[i] > risks[j] {
				risks[i], risks[j] = risks[j], risks[i]
			}
		}
	}

	return risks
}

// formatTimeRange converts an hour to a readable time range
func formatTimeRange(hour int) string {
	switch {
	case hour >= 5 && hour < 9:
		return "early_morning (5-9am)"
	case hour >= 9 && hour < 12:
		return "morning (9am-12pm)"
	case hour >= 12 && hour < 17:
		return "afternoon (12-5pm)"
	case hour >= 17 && hour < 21:
		return "evening (5-9pm)"
	case hour >= 21 || hour < 5:
		return "night (9pm-5am)"
	default:
		return "unknown"
	}
}
