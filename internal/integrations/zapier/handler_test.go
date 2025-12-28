package zapier

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"base-server/internal/integrations"
	"base-server/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestHandler(t *testing.T, ctrl *gomock.Controller) (*Handler, *MockIntegrationStore) {
	t.Helper()
	mockStore := NewMockIntegrationStore(ctrl)
	logger := observability.NewLogger()
	handler := NewHandler(mockStore, logger)
	return handler, mockStore
}

func setContextValues(c *gin.Context, accountID, apiKeyID uuid.UUID) {
	c.Set("Account-ID", accountID.String())
	c.Set("API-Key-ID", apiKeyID.String())
	c.Set("account_id", accountID.String())
}

func TestHandler_HandleMe(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler, _ := setupTestHandler(t, ctrl)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	accountID := uuid.New()
	setContextValues(c, accountID, uuid.New())

	handler.HandleMe(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AccountInfo
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, accountID.String(), response.ID)
	assert.Equal(t, "Connected Account", response.Name)
}

func TestHandler_HandleSubscribe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    SubscribeRequest
		setupMock      func(mockStore *MockIntegrationStore, accountID, apiKeyID uuid.UUID)
		expectedStatus int
	}{
		{
			name: "create subscription successfully",
			requestBody: SubscribeRequest{
				HookURL: "https://hooks.zapier.com/hooks/catch/123/abc",
				Event:   "user.created",
			},
			setupMock: func(mockStore *MockIntegrationStore, accountID, apiKeyID uuid.UUID) {
				mockStore.EXPECT().
					CreateIntegrationSubscription(
						gomock.Any(),
						gomock.Any(),
					).
					Return(integrations.Subscription{
						ID:              uuid.New(),
						AccountID:       accountID,
						APIKeyID:        &apiKeyID,
						IntegrationType: integrations.IntegrationZapier,
						TargetURL:       "https://hooks.zapier.com/hooks/catch/123/abc",
						EventType:       "user.created",
						Status:          "active",
					}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create subscription with campaign",
			requestBody: SubscribeRequest{
				HookURL:    "https://hooks.zapier.com/hooks/catch/456/def",
				Event:      "user.verified",
				CampaignID: strPtr(uuid.New().String()),
			},
			setupMock: func(mockStore *MockIntegrationStore, accountID, apiKeyID uuid.UUID) {
				mockStore.EXPECT().
					CreateIntegrationSubscription(
						gomock.Any(),
						gomock.Any(),
					).
					Return(integrations.Subscription{
						ID:        uuid.New(),
						AccountID: accountID,
						APIKeyID:  &apiKeyID,
						Status:    "active",
					}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "invalid event type",
			requestBody: SubscribeRequest{
				HookURL: "https://hooks.zapier.com/test",
				Event:   "invalid.event",
			},
			setupMock:      func(mockStore *MockIntegrationStore, accountID, apiKeyID uuid.UUID) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			handler, mockStore := setupTestHandler(t, ctrl)

			accountID := uuid.New()
			apiKeyID := uuid.New()
			tt.setupMock(mockStore, accountID, apiKeyID)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			body, _ := json.Marshal(tt.requestBody)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/zapier/subscribe", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			setContextValues(c, accountID, apiKeyID)

			handler.HandleSubscribe(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusCreated {
				var response SubscribeResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response.ID)
			}
		})
	}
}

func TestHandler_HandleUnsubscribe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		subscriptionID string
		setupMock      func(mockStore *MockIntegrationStore, accountID, subID uuid.UUID)
		expectedStatus int
	}{
		{
			name:           "unsubscribe successfully",
			subscriptionID: uuid.New().String(),
			setupMock: func(mockStore *MockIntegrationStore, accountID, subID uuid.UUID) {
				mockStore.EXPECT().
					GetIntegrationSubscriptionByID(gomock.Any(), subID).
					Return(integrations.Subscription{
						ID:        subID,
						AccountID: accountID,
					}, nil)

				mockStore.EXPECT().
					DeleteIntegrationSubscription(gomock.Any(), subID).
					Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "subscription belongs to different account",
			subscriptionID: uuid.New().String(),
			setupMock: func(mockStore *MockIntegrationStore, accountID, subID uuid.UUID) {
				mockStore.EXPECT().
					GetIntegrationSubscriptionByID(gomock.Any(), subID).
					Return(integrations.Subscription{
						ID:        subID,
						AccountID: uuid.New(), // Different account
					}, nil)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "invalid subscription id",
			subscriptionID: "not-a-uuid",
			setupMock:      func(mockStore *MockIntegrationStore, accountID, subID uuid.UUID) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			handler, mockStore := setupTestHandler(t, ctrl)

			accountID := uuid.New()
			subID, _ := uuid.Parse(tt.subscriptionID)
			tt.setupMock(mockStore, accountID, subID)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/zapier/subscribe/"+tt.subscriptionID, nil)
			c.Params = gin.Params{{Key: "id", Value: tt.subscriptionID}}
			setContextValues(c, accountID, uuid.New())

			handler.HandleUnsubscribe(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestHandler_HandleSampleData(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler, _ := setupTestHandler(t, ctrl)

	tests := []struct {
		name           string
		eventType      string
		expectedStatus int
	}{
		{
			name:           "user.created sample data",
			eventType:      "user.created",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "referral.created sample data",
			eventType:      "referral.created",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unknown event type",
			eventType:      "unknown.event",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/zapier/sample/"+tt.eventType, nil)
			c.Params = gin.Params{{Key: "event", Value: tt.eventType}}

			handler.HandleSampleData(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response []map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Len(t, response, 1)
				assert.Equal(t, tt.eventType, response[0]["event"])
			}
		})
	}
}

func TestHandler_HandleStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupMock      func(mockStore *MockIntegrationStore, accountID uuid.UUID)
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name: "connected with subscriptions",
			setupMock: func(mockStore *MockIntegrationStore, accountID uuid.UUID) {
				mockStore.EXPECT().
					GetIntegrationSubscriptionsByAccount(gomock.Any(), accountID, gomock.Any()).
					Return([]integrations.Subscription{
						{ID: uuid.New()},
						{ID: uuid.New()},
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response StatusResponse
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				assert.True(t, response.Connected)
				assert.Equal(t, 2, response.ActiveSubscriptions)
			},
		},
		{
			name: "not connected",
			setupMock: func(mockStore *MockIntegrationStore, accountID uuid.UUID) {
				mockStore.EXPECT().
					GetIntegrationSubscriptionsByAccount(gomock.Any(), accountID, gomock.Any()).
					Return([]integrations.Subscription{}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response StatusResponse
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				assert.False(t, response.Connected)
				assert.Equal(t, 0, response.ActiveSubscriptions)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			handler, mockStore := setupTestHandler(t, ctrl)

			accountID := uuid.New()
			tt.setupMock(mockStore, accountID)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Request = httptest.NewRequest(http.MethodGet, "/api/protected/integrations/zapier/status", nil)
			setContextValues(c, accountID, uuid.New())

			handler.HandleStatus(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.Bytes())
			}
		})
	}
}

func TestHandler_HandleSubscriptions(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler, mockStore := setupTestHandler(t, ctrl)

	accountID := uuid.New()
	now := time.Now()

	mockStore.EXPECT().
		GetIntegrationSubscriptionsByAccount(gomock.Any(), accountID, gomock.Any()).
		Return([]integrations.Subscription{
			{
				ID:              uuid.New(),
				EventType:       "user.created",
				Status:          "active",
				TriggerCount:    5,
				LastTriggeredAt: &now,
				CreatedAt:       now.Add(-24 * time.Hour),
			},
			{
				ID:           uuid.New(),
				EventType:    "user.verified",
				Status:       "active",
				TriggerCount: 0,
				CreatedAt:    now.Add(-1 * time.Hour),
			},
		}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest(http.MethodGet, "/api/protected/integrations/zapier/subscriptions", nil)
	setContextValues(c, accountID, uuid.New())

	handler.HandleSubscriptions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []SubscriptionResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Equal(t, "user.created", response[0].EventType)
	assert.Equal(t, 5, response[0].TriggerCount)
	assert.NotNil(t, response[0].LastTriggeredAt)
}

func TestHandler_HandleDisconnect(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler, mockStore := setupTestHandler(t, ctrl)

	accountID := uuid.New()
	subID1 := uuid.New()
	subID2 := uuid.New()

	// Mock getting subscriptions
	mockStore.EXPECT().
		GetIntegrationSubscriptionsByAccount(gomock.Any(), accountID, gomock.Any()).
		Return([]integrations.Subscription{
			{ID: subID1},
			{ID: subID2},
		}, nil)

	// Mock deleting each subscription
	mockStore.EXPECT().
		DeleteIntegrationSubscription(gomock.Any(), subID1).
		Return(nil)

	mockStore.EXPECT().
		DeleteIntegrationSubscription(gomock.Any(), subID2).
		Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest(http.MethodPost, "/api/protected/integrations/zapier/disconnect", nil)
	setContextValues(c, accountID, uuid.New())

	handler.HandleDisconnect(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["success"])
}

func TestIsValidEventType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		eventType string
		valid     bool
	}{
		{"user.created", true},
		{"user.verified", true},
		{"user.updated", true},
		{"user.deleted", true},
		{"user.position_changed", true},
		{"user.converted", true},
		{"referral.created", true},
		{"referral.verified", true},
		{"referral.converted", true},
		{"reward.earned", true},
		{"reward.delivered", true},
		{"reward.redeemed", true},
		{"campaign.milestone", true},
		{"campaign.launched", true},
		{"campaign.completed", true},
		{"email.sent", true},
		{"email.delivered", true},
		{"email.opened", true},
		{"email.clicked", true},
		{"email.bounced", true},
		{"invalid.event", false},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			t.Parallel()
			result := isValidEventType(tt.eventType)
			assert.Equal(t, tt.valid, result)
		})
	}
}

// Helper function
func strPtr(s string) *string {
	return &s
}
