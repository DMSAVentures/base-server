//go:build integration
// +build integration

package tests

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a campaign for email blast tests
func createTestCampaign(t *testing.T, token string) string {
	req := map[string]interface{}{
		"name":            "Test Campaign for Blasts",
		"slug":            generateTestCampaignSlug(),
		"type":            "waitlist",
		"form_config":     map[string]interface{}{},
		"referral_config": map[string]interface{}{},
		"email_config":    map[string]interface{}{},
		"branding_config": map[string]interface{}{},
	}

	resp := POST(t, "/api/v1/campaigns").
		WithToken(token).
		WithBody(req).
		Do()

	resp.RequireStatus(http.StatusCreated)
	return resp.JSON()["id"].(string)
}

// Helper to create a segment for email blast tests
func createTestSegment(t *testing.T, token, campaignID string) string {
	req := map[string]interface{}{
		"name":        "Test Segment for Blasts",
		"description": "Segment for testing email blasts",
		"filter_criteria": map[string]interface{}{
			"conditions": []map[string]interface{}{},
			"logic":      "and",
		},
	}

	resp := POST(t, fmt.Sprintf("/api/v1/campaigns/%s/segments", campaignID)).
		WithToken(token).
		WithBody(req).
		Do()

	resp.RequireStatus(http.StatusCreated)
	return resp.JSON()["id"].(string)
}

// Helper to create a blast email template for email blast tests
func createTestBlastEmailTemplate(t *testing.T, token string) string {
	testStore := setupTestStore(t)
	ctx := context.Background()

	// Get account ID from user's token
	resp := GET(t, "/api/protected/user").
		WithToken(token).
		Do()
	resp.RequireStatus(http.StatusOK)
	userID := resp.JSON()["external_id"].(string)

	// Get account ID from database
	var accountID uuid.UUID
	err := testStore.GetDB().GetContext(ctx, &accountID, `
		SELECT id FROM accounts WHERE owner_user_id = $1 LIMIT 1
	`, userID)
	require.NoError(t, err)

	// Create blast email template directly in database
	templateID := uuid.New()
	_, err = testStore.GetDB().ExecContext(ctx, `
		INSERT INTO blast_email_templates (id, account_id, name, subject, html_body)
		VALUES ($1, $2, $3, $4, $5)
	`, templateID, accountID, "Test Blast Template", "Test Subject", "<h1>Hello {{.first_name}}</h1>")
	require.NoError(t, err)

	return templateID.String()
}

func TestAPI_EmailBlast_Create(t *testing.T) {
	t.Parallel()
	// Use Team tier to have email_blasts feature enabled
	token := createAuthenticatedTestUserWithTeamTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID := createTestSegment(t, token, campaignID)
	templateID := createTestBlastEmailTemplate(t, token)

	tests := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
		validate       func(t *testing.T, resp *APIResponse)
	}{
		{
			name: "create email blast successfully",
			request: map[string]interface{}{
				"name":              "Test Email Blast",
				"segment_ids":      []string{segmentID},
				"blast_template_id": templateID,
				"subject":          "Test Subject",
				"batch_size":       100,
			},
			expectedStatus: http.StatusCreated,
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONFieldNotNil("id")
				resp.AssertJSONField("name", "Test Email Blast")
				resp.AssertJSONField("subject", "Test Subject")
				resp.AssertJSONField("status", "draft")
			},
		},
		{
			name: "create email blast with scheduled time",
			request: map[string]interface{}{
				"name":              "Scheduled Blast",
				"segment_ids":      []string{segmentID},
				"blast_template_id": templateID,
				"subject":          "Scheduled Subject",
				"scheduled_at":     time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			},
			expectedStatus: http.StatusCreated,
			validate: func(t *testing.T, resp *APIResponse) {
				resp.AssertJSONField("name", "Scheduled Blast")
				resp.AssertJSONFieldNotNil("scheduled_at")
			},
		},
		{
			name: "create fails without name",
			request: map[string]interface{}{
				"segment_ids":      []string{segmentID},
				"blast_template_id": templateID,
				"subject":          "Test Subject",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails without segment_ids",
			request: map[string]interface{}{
				"name":              "Test Blast",
				"blast_template_id": templateID,
				"subject":          "Test Subject",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails without blast_template_id",
			request: map[string]interface{}{
				"name":        "Test Blast",
				"segment_ids": []string{segmentID},
				"subject":     "Test Subject",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "create fails with invalid segment_id",
			request: map[string]interface{}{
				"name":              "Test Blast",
				"segment_ids":      []string{uuid.New().String()},
				"blast_template_id": templateID,
				"subject":          "Test Subject",
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "create fails with invalid template_id",
			request: map[string]interface{}{
				"name":              "Test Blast",
				"segment_ids":      []string{segmentID},
				"blast_template_id": uuid.New().String(),
				"subject":          "Test Subject",
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := POST(t, "/api/v1/blasts").
				WithToken(token).
				WithBody(tt.request).
				Do()

			resp.AssertStatus(tt.expectedStatus)
			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

func TestAPI_EmailBlast_Create_FeatureNotAvailable(t *testing.T) {
	t.Parallel()
	// Use Free tier which doesn't have email_blasts feature (only Team tier has it)
	token := createAuthenticatedTestUserWithFreeTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID := createTestSegment(t, token, campaignID)

	// Create a blast template even though we can't use it
	testStore := setupTestStore(t)
	ctx := context.Background()
	resp := GET(t, "/api/protected/user").
		WithToken(token).
		Do()
	resp.RequireStatus(http.StatusOK)
	userID := resp.JSON()["external_id"].(string)

	var accountID uuid.UUID
	err := testStore.GetDB().GetContext(ctx, &accountID, `
		SELECT id FROM accounts WHERE owner_user_id = $1 LIMIT 1
	`, userID)
	require.NoError(t, err)

	templateID := uuid.New()
	_, err = testStore.GetDB().ExecContext(ctx, `
		INSERT INTO blast_email_templates (id, account_id, name, subject, html_body)
		VALUES ($1, $2, $3, $4, $5)
	`, templateID, accountID, "Test Template", "Subject", "<h1>Hello</h1>")
	require.NoError(t, err)

	req := map[string]interface{}{
		"name":              "Test Email Blast",
		"segment_ids":      []string{segmentID},
		"blast_template_id": templateID.String(),
		"subject":          "Test Subject",
	}

	apiResp := POST(t, "/api/v1/blasts").
		WithToken(token).
		WithBody(req).
		Do()

	apiResp.AssertStatus(http.StatusForbidden)
}

func TestAPI_EmailBlast_List(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedTestUserWithTeamTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID := createTestSegment(t, token, campaignID)
	templateID := createTestBlastEmailTemplate(t, token)

	// Create a blast
	createReq := map[string]interface{}{
		"name":              "Test Blast for List",
		"segment_ids":      []string{segmentID},
		"blast_template_id": templateID,
		"subject":          "Test Subject",
	}

	createResp := POST(t, "/api/v1/blasts").
		WithToken(token).
		WithBody(createReq).
		Do()
	createResp.RequireStatus(http.StatusCreated)

	t.Run("list blasts successfully", func(t *testing.T) {
		resp := GET(t, "/api/v1/blasts").
			WithToken(token).
			Do()

		resp.RequireStatus(http.StatusOK)
		resp.AssertJSONFieldNotNil("blasts")
		resp.AssertJSONFieldNotNil("total")
		resp.AssertJSONFieldNotNil("page")
		resp.AssertJSONFieldNotNil("limit")

		blasts := resp.JSON()["blasts"].([]interface{})
		assert.GreaterOrEqual(t, len(blasts), 1)
	})

	t.Run("list blasts with pagination", func(t *testing.T) {
		resp := GET(t, "/api/v1/blasts?page=1&limit=10").
			WithToken(token).
			Do()

		resp.RequireStatus(http.StatusOK)
		assert.Equal(t, float64(1), resp.JSON()["page"])
		assert.Equal(t, float64(10), resp.JSON()["limit"])
	})
}

func TestAPI_EmailBlast_Get(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedTestUserWithTeamTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID := createTestSegment(t, token, campaignID)
	templateID := createTestBlastEmailTemplate(t, token)

	// Create a blast
	createReq := map[string]interface{}{
		"name":              "Test Blast for Get",
		"segment_ids":      []string{segmentID},
		"blast_template_id": templateID,
		"subject":          "Test Subject",
	}

	createResp := POST(t, "/api/v1/blasts").
		WithToken(token).
		WithBody(createReq).
		Do()
	createResp.RequireStatus(http.StatusCreated)
	blastID := createResp.JSON()["id"].(string)

	t.Run("get blast successfully", func(t *testing.T) {
		resp := GET(t, fmt.Sprintf("/api/v1/blasts/%s", blastID)).
			WithToken(token).
			Do()

		resp.RequireStatus(http.StatusOK)
		resp.AssertJSONField("id", blastID)
		resp.AssertJSONField("name", "Test Blast for Get")
	})

	t.Run("get blast returns 404 for non-existent", func(t *testing.T) {
		resp := GET(t, fmt.Sprintf("/api/v1/blasts/%s", uuid.New().String())).
			WithToken(token).
			Do()

		resp.AssertStatus(http.StatusNotFound)
	})
}

func TestAPI_EmailBlast_Update(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedTestUserWithTeamTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID := createTestSegment(t, token, campaignID)
	templateID := createTestBlastEmailTemplate(t, token)

	// Create a blast
	createReq := map[string]interface{}{
		"name":              "Original Name",
		"segment_ids":      []string{segmentID},
		"blast_template_id": templateID,
		"subject":          "Original Subject",
	}

	createResp := POST(t, "/api/v1/blasts").
		WithToken(token).
		WithBody(createReq).
		Do()
	createResp.RequireStatus(http.StatusCreated)
	blastID := createResp.JSON()["id"].(string)

	t.Run("update blast successfully", func(t *testing.T) {
		updateReq := map[string]interface{}{
			"name":    "Updated Name",
			"subject": "Updated Subject",
		}

		resp := PUT(t, fmt.Sprintf("/api/v1/blasts/%s", blastID)).
			WithToken(token).
			WithBody(updateReq).
			Do()

		resp.RequireStatus(http.StatusOK)
		resp.AssertJSONField("name", "Updated Name")
		resp.AssertJSONField("subject", "Updated Subject")
	})

	t.Run("update returns 404 for non-existent blast", func(t *testing.T) {
		updateReq := map[string]interface{}{
			"name": "New Name",
		}

		resp := PUT(t, fmt.Sprintf("/api/v1/blasts/%s", uuid.New().String())).
			WithToken(token).
			WithBody(updateReq).
			Do()

		resp.AssertStatus(http.StatusNotFound)
	})
}

func TestAPI_EmailBlast_Delete(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedTestUserWithTeamTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID := createTestSegment(t, token, campaignID)
	templateID := createTestBlastEmailTemplate(t, token)

	// Create a blast
	createReq := map[string]interface{}{
		"name":              "Blast to Delete",
		"segment_ids":      []string{segmentID},
		"blast_template_id": templateID,
		"subject":          "Test Subject",
	}

	createResp := POST(t, "/api/v1/blasts").
		WithToken(token).
		WithBody(createReq).
		Do()
	createResp.RequireStatus(http.StatusCreated)
	blastID := createResp.JSON()["id"].(string)

	t.Run("delete blast successfully", func(t *testing.T) {
		resp := DELETE(t, fmt.Sprintf("/api/v1/blasts/%s", blastID)).
			WithToken(token).
			Do()

		resp.AssertStatus(http.StatusNoContent)

		// Verify blast is deleted
		getResp := GET(t, fmt.Sprintf("/api/v1/blasts/%s", blastID)).
			WithToken(token).
			Do()
		getResp.AssertStatus(http.StatusNotFound)
	})

	t.Run("delete returns 404 for non-existent blast", func(t *testing.T) {
		resp := DELETE(t, fmt.Sprintf("/api/v1/blasts/%s", uuid.New().String())).
			WithToken(token).
			Do()

		resp.AssertStatus(http.StatusNotFound)
	})
}

func TestAPI_EmailBlast_Schedule(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedTestUserWithTeamTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID := createTestSegment(t, token, campaignID)
	templateID := createTestBlastEmailTemplate(t, token)

	// Create a blast
	createReq := map[string]interface{}{
		"name":              "Blast to Schedule",
		"segment_ids":      []string{segmentID},
		"blast_template_id": templateID,
		"subject":          "Test Subject",
	}

	createResp := POST(t, "/api/v1/blasts").
		WithToken(token).
		WithBody(createReq).
		Do()
	createResp.RequireStatus(http.StatusCreated)
	blastID := createResp.JSON()["id"].(string)

	t.Run("schedule blast successfully", func(t *testing.T) {
		futureTime := time.Now().Add(24 * time.Hour)
		scheduleReq := map[string]interface{}{
			"scheduled_at": futureTime.Format(time.RFC3339),
		}

		resp := POST(t, fmt.Sprintf("/api/v1/blasts/%s/schedule", blastID)).
			WithToken(token).
			WithBody(scheduleReq).
			Do()

		resp.RequireStatus(http.StatusOK)
		resp.AssertJSONField("status", "scheduled")
		resp.AssertJSONFieldNotNil("scheduled_at")
	})

	t.Run("schedule fails with past time", func(t *testing.T) {
		// Create another blast
		createResp2 := POST(t, "/api/v1/blasts").
			WithToken(token).
			WithBody(createReq).
			Do()
		createResp2.RequireStatus(http.StatusCreated)
		blastID2 := createResp2.JSON()["id"].(string)

		pastTime := time.Now().Add(-1 * time.Hour)
		scheduleReq := map[string]interface{}{
			"scheduled_at": pastTime.Format(time.RFC3339),
		}

		resp := POST(t, fmt.Sprintf("/api/v1/blasts/%s/schedule", blastID2)).
			WithToken(token).
			WithBody(scheduleReq).
			Do()

		resp.AssertStatus(http.StatusBadRequest)
	})
}

func TestAPI_EmailBlast_Cancel(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedTestUserWithTeamTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID := createTestSegment(t, token, campaignID)
	templateID := createTestBlastEmailTemplate(t, token)

	// Create a blast
	createReq := map[string]interface{}{
		"name":              "Blast to Cancel",
		"segment_ids":      []string{segmentID},
		"blast_template_id": templateID,
		"subject":          "Test Subject",
	}

	createResp := POST(t, "/api/v1/blasts").
		WithToken(token).
		WithBody(createReq).
		Do()
	createResp.RequireStatus(http.StatusCreated)
	blastID := createResp.JSON()["id"].(string)

	t.Run("cancel draft blast successfully", func(t *testing.T) {
		resp := POST(t, fmt.Sprintf("/api/v1/blasts/%s/cancel", blastID)).
			WithToken(token).
			Do()

		resp.RequireStatus(http.StatusOK)
		resp.AssertJSONField("status", "cancelled")
	})
}

func TestAPI_EmailBlast_Analytics(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedTestUserWithTeamTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID := createTestSegment(t, token, campaignID)
	templateID := createTestBlastEmailTemplate(t, token)

	// Create a blast
	createReq := map[string]interface{}{
		"name":              "Blast for Analytics",
		"segment_ids":      []string{segmentID},
		"blast_template_id": templateID,
		"subject":          "Test Subject",
	}

	createResp := POST(t, "/api/v1/blasts").
		WithToken(token).
		WithBody(createReq).
		Do()
	createResp.RequireStatus(http.StatusCreated)
	blastID := createResp.JSON()["id"].(string)

	t.Run("get blast analytics successfully", func(t *testing.T) {
		resp := GET(t, fmt.Sprintf("/api/v1/blasts/%s/analytics", blastID)).
			WithToken(token).
			Do()

		resp.RequireStatus(http.StatusOK)
		resp.AssertJSONField("blast_id", blastID)
		resp.AssertJSONFieldExists("total_recipients")
		resp.AssertJSONFieldExists("sent")
		resp.AssertJSONFieldExists("open_rate")
		resp.AssertJSONFieldExists("click_rate")
	})

	t.Run("get analytics returns 404 for non-existent blast", func(t *testing.T) {
		resp := GET(t, fmt.Sprintf("/api/v1/blasts/%s/analytics", uuid.New().String())).
			WithToken(token).
			Do()

		resp.AssertStatus(http.StatusNotFound)
	})
}

func TestAPI_EmailBlast_Recipients(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedTestUserWithTeamTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID := createTestSegment(t, token, campaignID)
	templateID := createTestBlastEmailTemplate(t, token)

	// Create a blast
	createReq := map[string]interface{}{
		"name":              "Blast for Recipients List",
		"segment_ids":      []string{segmentID},
		"blast_template_id": templateID,
		"subject":          "Test Subject",
	}

	createResp := POST(t, "/api/v1/blasts").
		WithToken(token).
		WithBody(createReq).
		Do()
	createResp.RequireStatus(http.StatusCreated)
	blastID := createResp.JSON()["id"].(string)

	t.Run("list blast recipients successfully", func(t *testing.T) {
		resp := GET(t, fmt.Sprintf("/api/v1/blasts/%s/recipients", blastID)).
			WithToken(token).
			Do()

		resp.RequireStatus(http.StatusOK)
		resp.AssertJSONFieldNotNil("recipients")
		resp.AssertJSONFieldNotNil("total")
		resp.AssertJSONFieldNotNil("page")
		resp.AssertJSONFieldNotNil("limit")
	})

	t.Run("list recipients with pagination", func(t *testing.T) {
		resp := GET(t, fmt.Sprintf("/api/v1/blasts/%s/recipients?page=1&limit=10", blastID)).
			WithToken(token).
			Do()

		resp.RequireStatus(http.StatusOK)
		assert.Equal(t, float64(1), resp.JSON()["page"])
		assert.Equal(t, float64(10), resp.JSON()["limit"])
	})
}

func TestAPI_EmailBlast_MultipleSegments(t *testing.T) {
	t.Parallel()
	token := createAuthenticatedTestUserWithTeamTier(t)

	// Create prerequisite data
	campaignID := createTestCampaign(t, token)
	segmentID1 := createTestSegment(t, token, campaignID)
	segmentID2 := createTestSegment(t, token, campaignID)
	templateID := createTestBlastEmailTemplate(t, token)

	t.Run("create blast with multiple segments", func(t *testing.T) {
		req := map[string]interface{}{
			"name":              "Multi-Segment Blast",
			"segment_ids":      []string{segmentID1, segmentID2},
			"blast_template_id": templateID,
			"subject":          "Multi-Segment Subject",
		}

		resp := POST(t, "/api/v1/blasts").
			WithToken(token).
			WithBody(req).
			Do()

		resp.RequireStatus(http.StatusCreated)
		resp.AssertJSONField("name", "Multi-Segment Blast")

		// Verify segment_ids are stored
		segmentIDs := resp.JSON()["segment_ids"].([]interface{})
		assert.Len(t, segmentIDs, 2)
	})
}
