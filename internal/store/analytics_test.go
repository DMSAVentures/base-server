package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStore_GetSignupsOverTime(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()

	// Create test account and campaign
	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Analytics Test", "analytics-test")

	// Create users at specific times for testing
	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	twoDaysAgo := now.AddDate(0, 0, -2)
	lastWeek := now.AddDate(0, 0, -7)
	lastMonth := now.AddDate(0, -1, 0)

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

	// Create users at different times
	createUserAtTime("today1@example.com", now.Add(-1*time.Hour))
	createUserAtTime("today2@example.com", now.Add(-2*time.Hour))
	createUserAtTime("today3@example.com", now.Add(-3*time.Hour))
	createUserAtTime("yesterday1@example.com", yesterday)
	createUserAtTime("yesterday2@example.com", yesterday.Add(-1*time.Hour))
	createUserAtTime("twodays1@example.com", twoDaysAgo)
	createUserAtTime("lastweek1@example.com", lastWeek)
	createUserAtTime("lastmonth1@example.com", lastMonth)

	t.Run("daily period", func(t *testing.T) {
		from := now.AddDate(0, 0, -3)
		to := now

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
		for _, r := range results {
			truncatedToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			truncatedYesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)

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
		from := now.Add(-4 * time.Hour)
		to := now

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
		from := now.AddDate(0, -2, 0)
		to := now

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
			if dateUTC.Year() == now.Year() && dateUTC.Month() == now.Month() && r.Count > 0 {
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
		otherCampaign := createTestCampaign(t, testDB, account.ID, "Other Campaign", "other-campaign")

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

func TestStore_GetSignupsOverTime_TimezoneHandling(t *testing.T) {
	testDB := SetupTestDB(t, TestDBTypePostgres)
	defer testDB.Close()
	testDB.Truncate(t)

	ctx := context.Background()

	account := createTestAccount(t, testDB)
	campaign := createTestCampaign(t, testDB, account.ID, "Timezone Test", "timezone-test")

	// Create a user at a specific UTC time
	specificTime := time.Date(2025, 12, 20, 14, 30, 0, 0, time.UTC)
	_, err := testDB.db.ExecContext(ctx, `
		INSERT INTO waitlist_users (
			id, campaign_id, email, referral_code, position, original_position,
			terms_accepted, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, uuid.New(), campaign.ID, "timezone@example.com", "TZREF1", 1, 1, true, specificTime, specificTime)
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
