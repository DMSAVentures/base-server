package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStore_GetSignupsOverTime(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	// Create test account and campaign with unique slugs
	uniqueID := uuid.New().String()[:8]
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Analytics Test "+uniqueID, "analytics-test-"+uniqueID)

	// Create users at specific times for testing
	// Use noon as the base time to avoid date boundary issues
	now := time.Now().UTC()
	todayNoon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
	yesterdayNoon := todayNoon.AddDate(0, 0, -1)
	twoDaysAgoNoon := todayNoon.AddDate(0, 0, -2)
	lastWeekNoon := todayNoon.AddDate(0, 0, -7)
	lastMonthNoon := todayNoon.AddDate(0, -1, 0)

	// Helper to create user with specific created_at
	createUserAtTime := func(email string, createdAt time.Time) {
		refCode := fmt.Sprintf("REF%s", uuid.New().String()[:8])
		_, err := testDB.db.ExecContext(ctx, `
			INSERT INTO waitlist_users (
				id, campaign_id, email, referral_code, position, original_position,
				terms_accepted, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, uuid.New(), campaign.ID, email, refCode, 1, 1, true, createdAt, createdAt)
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}
	}

	// Create users at different times (using minute offsets to stay within same day)
	createUserAtTime(uniqueID+"today1@example.com", todayNoon.Add(-1*time.Minute))
	createUserAtTime(uniqueID+"today2@example.com", todayNoon.Add(-2*time.Minute))
	createUserAtTime(uniqueID+"today3@example.com", todayNoon.Add(-3*time.Minute))
	createUserAtTime(uniqueID+"yesterday1@example.com", yesterdayNoon)
	createUserAtTime(uniqueID+"yesterday2@example.com", yesterdayNoon.Add(-1*time.Minute))
	createUserAtTime(uniqueID+"twodays1@example.com", twoDaysAgoNoon)
	createUserAtTime(uniqueID+"lastweek1@example.com", lastWeekNoon)
	createUserAtTime(uniqueID+"lastmonth1@example.com", lastMonthNoon)

	t.Run("daily period", func(t *testing.T) {
		from := todayNoon.AddDate(0, 0, -3)
		to := todayNoon.AddDate(0, 0, 1) // Include today

		results, err := testDB.Store.GetSignupsOverTime(ctx, campaign.ID, from, to, "day")
		if err != nil {
			t.Fatalf("GetSignupsOverTime() error = %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected results, got none")
		}

		// Verify we have data for today
		todayCount := 0
		yesterdayCount := 0
		truncatedToday := time.Date(todayNoon.Year(), todayNoon.Month(), todayNoon.Day(), 0, 0, 0, 0, time.UTC)
		truncatedYesterday := time.Date(yesterdayNoon.Year(), yesterdayNoon.Month(), yesterdayNoon.Day(), 0, 0, 0, 0, time.UTC)

		for _, r := range results {
			if r.Date.Equal(truncatedToday) {
				todayCount = r.Count
			}
			if r.Date.Equal(truncatedYesterday) {
				yesterdayCount = r.Count
			}
		}

		if todayCount != 3 {
			t.Errorf("Expected 3 signups today, got %d", todayCount)
		}
		if yesterdayCount != 2 {
			t.Errorf("Expected 2 signups yesterday, got %d", yesterdayCount)
		}
	})

	t.Run("hourly period", func(t *testing.T) {
		// Use a time range that includes todayNoon
		from := todayNoon.Add(-1 * time.Hour)
		to := todayNoon.Add(1 * time.Hour)

		results, err := testDB.Store.GetSignupsOverTime(ctx, campaign.ID, from, to, "hour")
		if err != nil {
			t.Fatalf("GetSignupsOverTime() error = %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected results, got none")
		}

		// Should have at least some hourly data
		totalCount := 0
		for _, r := range results {
			totalCount += r.Count
		}

		if totalCount < 1 {
			t.Errorf("Expected at least 1 signup in hourly data, got %d", totalCount)
		}
	})

	t.Run("weekly period", func(t *testing.T) {
		from := now.AddDate(0, 0, -14)
		to := now

		results, err := testDB.Store.GetSignupsOverTime(ctx, campaign.ID, from, to, "week")
		if err != nil {
			t.Fatalf("GetSignupsOverTime() error = %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected results, got none")
		}

		// Verify data is grouped by week (convert to UTC for comparison)
		for _, r := range results {
			dateUTC := r.Date.UTC()
			// DATE_TRUNC('week', ...) in PostgreSQL returns Monday
			if dateUTC.Weekday() != time.Monday {
				t.Errorf("Expected week to start on Monday, got %v (date: %v)", dateUTC.Weekday(), dateUTC)
			}
		}
	})

	t.Run("monthly period", func(t *testing.T) {
		from := todayNoon.AddDate(0, -2, 0)
		to := todayNoon.AddDate(0, 0, 1) // Include today

		results, err := testDB.Store.GetSignupsOverTime(ctx, campaign.ID, from, to, "month")
		if err != nil {
			t.Fatalf("GetSignupsOverTime() error = %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected results, got none")
		}

		// Verify data is grouped by month (day should be 1, convert to UTC)
		for _, r := range results {
			dateUTC := r.Date.UTC()
			if dateUTC.Day() != 1 {
				t.Errorf("Expected month to start on day 1, got day %d (date: %v)", dateUTC.Day(), dateUTC)
			}
		}

		// Should have data for current month
		currentMonthFound := false
		for _, r := range results {
			dateUTC := r.Date.UTC()
			if dateUTC.Year() == todayNoon.Year() && dateUTC.Month() == todayNoon.Month() && r.Count > 0 {
				currentMonthFound = true
				break
			}
		}
		if !currentMonthFound {
			t.Error("Expected to find signups in current month")
		}
	})

	t.Run("empty result for future dates", func(t *testing.T) {
		from := now.AddDate(1, 0, 0) // 1 year in future
		to := now.AddDate(1, 0, 7)

		results, err := testDB.Store.GetSignupsOverTime(ctx, campaign.ID, from, to, "day")
		if err != nil {
			t.Fatalf("GetSignupsOverTime() error = %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected no results for future dates, got %d", len(results))
		}
	})

	t.Run("invalid period defaults to day", func(t *testing.T) {
		from := now.AddDate(0, 0, -3)
		to := now

		results, err := testDB.Store.GetSignupsOverTime(ctx, campaign.ID, from, to, "invalid")
		if err != nil {
			t.Fatalf("GetSignupsOverTime() error = %v", err)
		}

		// Should still return results (defaulting to day)
		if len(results) == 0 {
			t.Fatal("Expected results with invalid period (should default to day)")
		}
	})

	t.Run("different campaign returns no data", func(t *testing.T) {
		otherUniqueID := uuid.New().String()[:8]
		otherCampaign := createTestCampaign(t, testDB, account.ID, "Other Campaign "+otherUniqueID, "other-campaign-"+otherUniqueID)

		from := now.AddDate(0, 0, -7)
		to := now

		results, err := testDB.Store.GetSignupsOverTime(ctx, otherCampaign.ID, from, to, "day")
		if err != nil {
			t.Fatalf("GetSignupsOverTime() error = %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected no results for different campaign, got %d", len(results))
		}
	})
}

func TestStore_GetSignupsBySource(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	// Create test account and campaign with unique slugs
	uniqueID := uuid.New().String()[:8]
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Source Analytics Test "+uniqueID, "source-analytics-test-"+uniqueID)

	// Create users at specific times with different UTM sources
	// Use noon as the base time to avoid date boundary issues
	now := time.Now().UTC()
	todayNoon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
	yesterdayNoon := todayNoon.AddDate(0, 0, -1)
	twoDaysAgoNoon := todayNoon.AddDate(0, 0, -2)

	// Helper to create user with specific created_at and utm_source
	createUserWithSource := func(email string, createdAt time.Time, utmSource *string) {
		refCode := fmt.Sprintf("REF%s", uuid.New().String()[:8])
		var utmVal interface{} = nil
		if utmSource != nil {
			utmVal = *utmSource
		}
		_, err := testDB.db.ExecContext(ctx, `
			INSERT INTO waitlist_users (
				id, campaign_id, email, referral_code, position, original_position,
				terms_accepted, created_at, updated_at, utm_source
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, uuid.New(), campaign.ID, email, refCode, 1, 1, true, createdAt, createdAt, utmVal)
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}
	}

	// Create users with different sources at different times
	google := "google"
	facebook := "facebook"

	// Today: 2 google, 1 facebook, 1 null (using minute offsets to stay within same day)
	createUserWithSource(uniqueID+"today-google1@example.com", todayNoon.Add(-1*time.Minute), &google)
	createUserWithSource(uniqueID+"today-google2@example.com", todayNoon.Add(-2*time.Minute), &google)
	createUserWithSource(uniqueID+"today-facebook1@example.com", todayNoon.Add(-3*time.Minute), &facebook)
	createUserWithSource(uniqueID+"today-null1@example.com", todayNoon.Add(-4*time.Minute), nil)

	// Yesterday: 1 google, 2 facebook
	createUserWithSource(uniqueID+"yesterday-google1@example.com", yesterdayNoon, &google)
	createUserWithSource(uniqueID+"yesterday-facebook1@example.com", yesterdayNoon.Add(-1*time.Minute), &facebook)
	createUserWithSource(uniqueID+"yesterday-facebook2@example.com", yesterdayNoon.Add(-2*time.Minute), &facebook)

	// Two days ago: 1 null
	createUserWithSource(uniqueID+"twodays-null1@example.com", twoDaysAgoNoon, nil)

	t.Run("daily period with multiple sources", func(t *testing.T) {
		from := todayNoon.AddDate(0, 0, -3)
		to := todayNoon.AddDate(0, 0, 1) // Include today

		results, err := testDB.Store.GetSignupsBySource(ctx, campaign.ID, from, to, "day")
		if err != nil {
			t.Fatalf("GetSignupsBySource() error = %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected results, got none")
		}

		// Count by source
		sourceCounts := make(map[string]int)
		for _, r := range results {
			source := ""
			if r.UTMSource != nil {
				source = *r.UTMSource
			}
			sourceCounts[source] += r.Count
		}

		// Verify totals per source
		if sourceCounts["google"] != 3 {
			t.Errorf("Expected 3 google signups, got %d", sourceCounts["google"])
		}
		if sourceCounts["facebook"] != 3 {
			t.Errorf("Expected 3 facebook signups, got %d", sourceCounts["facebook"])
		}
		if sourceCounts[""] != 2 {
			t.Errorf("Expected 2 null source signups, got %d", sourceCounts[""])
		}
	})

	t.Run("daily period - verify date grouping", func(t *testing.T) {
		from := todayNoon.AddDate(0, 0, -3)
		to := todayNoon.AddDate(0, 0, 1) // Include today

		results, err := testDB.Store.GetSignupsBySource(ctx, campaign.ID, from, to, "day")
		if err != nil {
			t.Fatalf("GetSignupsBySource() error = %v", err)
		}

		// Group by date
		dateSourceCounts := make(map[string]map[string]int)
		for _, r := range results {
			dateKey := r.Date.UTC().Format("2006-01-02")
			source := ""
			if r.UTMSource != nil {
				source = *r.UTMSource
			}
			if dateSourceCounts[dateKey] == nil {
				dateSourceCounts[dateKey] = make(map[string]int)
			}
			dateSourceCounts[dateKey][source] = r.Count
		}

		todayKey := time.Date(todayNoon.Year(), todayNoon.Month(), todayNoon.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		yesterdayKey := time.Date(yesterdayNoon.Year(), yesterdayNoon.Month(), yesterdayNoon.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")

		// Verify today's counts
		if dateSourceCounts[todayKey]["google"] != 2 {
			t.Errorf("Expected 2 google signups today, got %d", dateSourceCounts[todayKey]["google"])
		}
		if dateSourceCounts[todayKey]["facebook"] != 1 {
			t.Errorf("Expected 1 facebook signup today, got %d", dateSourceCounts[todayKey]["facebook"])
		}

		// Verify yesterday's counts
		if dateSourceCounts[yesterdayKey]["google"] != 1 {
			t.Errorf("Expected 1 google signup yesterday, got %d", dateSourceCounts[yesterdayKey]["google"])
		}
		if dateSourceCounts[yesterdayKey]["facebook"] != 2 {
			t.Errorf("Expected 2 facebook signups yesterday, got %d", dateSourceCounts[yesterdayKey]["facebook"])
		}
	})

	t.Run("weekly period", func(t *testing.T) {
		from := now.AddDate(0, 0, -14)
		to := now

		results, err := testDB.Store.GetSignupsBySource(ctx, campaign.ID, from, to, "week")
		if err != nil {
			t.Fatalf("GetSignupsBySource() error = %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected results, got none")
		}

		// Verify data is grouped by week (convert to UTC for comparison)
		for _, r := range results {
			dateUTC := r.Date.UTC()
			if dateUTC.Weekday() != time.Monday {
				t.Errorf("Expected week to start on Monday, got %v (date: %v)", dateUTC.Weekday(), dateUTC)
			}
		}
	})

	t.Run("monthly period", func(t *testing.T) {
		from := now.AddDate(0, -2, 0)
		to := now

		results, err := testDB.Store.GetSignupsBySource(ctx, campaign.ID, from, to, "month")
		if err != nil {
			t.Fatalf("GetSignupsBySource() error = %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected results, got none")
		}

		// Verify data is grouped by month (day should be 1, convert to UTC)
		for _, r := range results {
			dateUTC := r.Date.UTC()
			if dateUTC.Day() != 1 {
				t.Errorf("Expected month to start on day 1, got day %d (date: %v)", dateUTC.Day(), dateUTC)
			}
		}
	})

	t.Run("empty result for future dates", func(t *testing.T) {
		from := now.AddDate(1, 0, 0) // 1 year in future
		to := now.AddDate(1, 0, 7)

		results, err := testDB.Store.GetSignupsBySource(ctx, campaign.ID, from, to, "day")
		if err != nil {
			t.Fatalf("GetSignupsBySource() error = %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected no results for future dates, got %d", len(results))
		}
	})

	t.Run("invalid period defaults to day", func(t *testing.T) {
		from := now.AddDate(0, 0, -3)
		to := now

		results, err := testDB.Store.GetSignupsBySource(ctx, campaign.ID, from, to, "invalid")
		if err != nil {
			t.Fatalf("GetSignupsBySource() error = %v", err)
		}

		// Should still return results (defaulting to day)
		if len(results) == 0 {
			t.Fatal("Expected results with invalid period (should default to day)")
		}
	})

	t.Run("different campaign returns no data", func(t *testing.T) {
		otherUniqueID := uuid.New().String()[:8]
		otherCampaign := createTestCampaign(t, testDB, account.ID, "Other Campaign 2 "+otherUniqueID, "other-campaign-2-"+otherUniqueID)

		from := now.AddDate(0, 0, -7)
		to := now

		results, err := testDB.Store.GetSignupsBySource(ctx, otherCampaign.ID, from, to, "day")
		if err != nil {
			t.Fatalf("GetSignupsBySource() error = %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected no results for different campaign, got %d", len(results))
		}
	})
}

func TestStore_GetSignupsOverTime_TimezoneHandling(t *testing.T) {
	t.Parallel()
	testDB := SetupTestDB(t, TestDBTypePostgres)

	ctx := context.Background()

	uniqueID := uuid.New().String()[:8]
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Timezone Test "+uniqueID, "timezone-test-"+uniqueID)

	// Create a user at a specific UTC time
	specificTime := time.Date(2025, 12, 20, 14, 30, 0, 0, time.UTC)
	_, err := testDB.db.ExecContext(ctx, `
		INSERT INTO waitlist_users (
			id, campaign_id, email, referral_code, position, original_position,
			terms_accepted, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, uuid.New(), campaign.ID, uniqueID+"timezone@example.com", "TZREF"+uniqueID, 1, 1, true, specificTime, specificTime)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Run("daily truncation is correct", func(t *testing.T) {
		from := time.Date(2025, 12, 19, 0, 0, 0, 0, time.UTC)
		to := time.Date(2025, 12, 21, 23, 59, 59, 0, time.UTC)

		results, err := testDB.Store.GetSignupsOverTime(ctx, campaign.ID, from, to, "day")
		if err != nil {
			t.Fatalf("GetSignupsOverTime() error = %v", err)
		}

		// Should have exactly one result on Dec 20 (comparing in UTC)
		found := false
		for _, r := range results {
			// Convert to UTC for comparison
			dateUTC := r.Date.UTC()
			if dateUTC.Year() == 2025 &&
				dateUTC.Month() == time.December &&
				dateUTC.Day() == 20 &&
				r.Count == 1 {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Expected 1 signup on Dec 20, 2025. Results: %+v", results)
		}
	})

	t.Run("monthly truncation is correct", func(t *testing.T) {
		from := time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

		results, err := testDB.Store.GetSignupsOverTime(ctx, campaign.ID, from, to, "month")
		if err != nil {
			t.Fatalf("GetSignupsOverTime() error = %v", err)
		}

		// Should have data in December, not November (compare in UTC)
		decemberFound := false
		novemberHasData := false

		for _, r := range results {
			dateUTC := r.Date.UTC()
			if dateUTC.Month() == time.December && r.Count > 0 {
				decemberFound = true
			}
			if dateUTC.Month() == time.November && r.Count > 0 {
				novemberHasData = true
			}
		}

		if !decemberFound {
			t.Error("Expected to find signup in December")
		}
		if novemberHasData {
			t.Error("Did not expect any signups in November")
		}
	})
}
