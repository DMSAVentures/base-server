package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func TestSignup_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	email := "test@example.com"
	firstName := "John"
	lastName := "Doe"
	password := "password123"
	userID := uuid.New()
	stripeCustomerID := "cus_123"

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)
	mockStore.EXPECT().CreateUserOnEmailSignup(gomock.Any(), firstName, lastName, email, gomock.Any()).
		Return(store.User{ID: userID, FirstName: firstName, LastName: lastName}, nil)
	mockBilling.EXPECT().CreateStripeCustomer(gomock.Any(), email).Return(stripeCustomerID, nil)
	mockStore.EXPECT().UpdateStripeCustomerIDByUserID(gomock.Any(), userID, stripeCustomerID).Return(nil)
	mockBilling.EXPECT().CreateFreeSubscription(gomock.Any(), stripeCustomerID).Return(nil)

	result, err := processor.Signup(ctx, firstName, lastName, email, password)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Email != email {
		t.Errorf("expected email %s, got %s", email, result.Email)
	}
	if result.FirstName != firstName {
		t.Errorf("expected firstName %s, got %s", firstName, result.FirstName)
	}
}

func TestSignup_EmailAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	email := "existing@example.com"

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(true, nil)

	_, err := processor.Signup(ctx, "John", "Doe", email, "password123")

	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestSignup_CheckEmailError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	email := "test@example.com"

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, errors.New("db error"))

	_, err := processor.Signup(ctx, "John", "Doe", email, "password123")

	if !errors.Is(err, ErrFailedSignup) {
		t.Errorf("expected ErrFailedSignup, got %v", err)
	}
}

func TestSignup_CreateUserError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	email := "test@example.com"

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)
	mockStore.EXPECT().CreateUserOnEmailSignup(gomock.Any(), gomock.Any(), gomock.Any(), email, gomock.Any()).
		Return(store.User{}, errors.New("db error"))

	_, err := processor.Signup(ctx, "John", "Doe", email, "password123")

	if !errors.Is(err, ErrFailedSignup) {
		t.Errorf("expected ErrFailedSignup, got %v", err)
	}
}

func TestSignup_StripeCustomerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	email := "test@example.com"
	userID := uuid.New()

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)
	mockStore.EXPECT().CreateUserOnEmailSignup(gomock.Any(), gomock.Any(), gomock.Any(), email, gomock.Any()).
		Return(store.User{ID: userID}, nil)
	mockBilling.EXPECT().CreateStripeCustomer(gomock.Any(), email).Return("", errors.New("stripe error"))

	_, err := processor.Signup(ctx, "John", "Doe", email, "password123")

	if !errors.Is(err, ErrFailedSignup) {
		t.Errorf("expected ErrFailedSignup, got %v", err)
	}
}

func TestSignup_UpdateStripeCustomerIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	email := "test@example.com"
	userID := uuid.New()
	stripeCustomerID := "cus_123"

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)
	mockStore.EXPECT().CreateUserOnEmailSignup(gomock.Any(), gomock.Any(), gomock.Any(), email, gomock.Any()).
		Return(store.User{ID: userID}, nil)
	mockBilling.EXPECT().CreateStripeCustomer(gomock.Any(), email).Return(stripeCustomerID, nil)
	mockStore.EXPECT().UpdateStripeCustomerIDByUserID(gomock.Any(), userID, stripeCustomerID).Return(errors.New("db error"))

	_, err := processor.Signup(ctx, "John", "Doe", email, "password123")

	if !errors.Is(err, ErrFailedSignup) {
		t.Errorf("expected ErrFailedSignup, got %v", err)
	}
}

func TestSignup_CreateFreeSubscriptionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	email := "test@example.com"
	firstName := "John"
	lastName := "Doe"
	password := "password123"
	userID := uuid.New()
	stripeCustomerID := "cus_123"

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)
	mockStore.EXPECT().CreateUserOnEmailSignup(gomock.Any(), firstName, lastName, email, gomock.Any()).
		Return(store.User{ID: userID, FirstName: firstName, LastName: lastName}, nil)
	mockBilling.EXPECT().CreateStripeCustomer(gomock.Any(), email).Return(stripeCustomerID, nil)
	mockStore.EXPECT().UpdateStripeCustomerIDByUserID(gomock.Any(), userID, stripeCustomerID).Return(nil)
	mockBilling.EXPECT().CreateFreeSubscription(gomock.Any(), stripeCustomerID).Return(errors.New("free subscription error"))

	_, err := processor.Signup(ctx, firstName, lastName, email, password)

	if !errors.Is(err, ErrFailedSignup) {
		t.Errorf("expected ErrFailedSignup, got %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	authConfig := AuthConfig{
		Email: EmailConfig{JWTSecret: "test-secret-key-that-is-long-enough"},
	}
	processor := New(mockStore, authConfig, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	email := "test@example.com"
	password := "password123"
	authID := uuid.New()
	userID := uuid.New()
	accountID := uuid.New()

	// Generate a proper bcrypt hash for the password
	hashedBytes, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	hashedPassword := string(hashedBytes)

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(true, nil)
	mockStore.EXPECT().GetCredentialsByEmail(gomock.Any(), email).Return(store.EmailAuth{
		AuthID:         authID,
		HashedPassword: hashedPassword,
	}, nil)
	mockStore.EXPECT().GetUserByAuthID(gomock.Any(), authID).Return(store.AuthenticatedUser{
		UserID:    userID,
		AccountID: accountID,
	}, nil)

	token, err := processor.Login(ctx, email, password)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if token == "" {
		t.Error("expected token to be non-empty")
	}
}

func TestLogin_EmailDoesNotExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	email := "nonexistent@example.com"

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(false, nil)

	_, err := processor.Login(ctx, email, "password123")

	if !errors.Is(err, ErrEmailDoesNotExist) {
		t.Errorf("expected ErrEmailDoesNotExist, got %v", err)
	}
}

func TestLogin_IncorrectPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	email := "test@example.com"
	authID := uuid.New()

	// Generate a hash for "password123"
	hashedBytes, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	hashedPassword := string(hashedBytes)

	mockStore.EXPECT().CheckIfEmailExists(gomock.Any(), email).Return(true, nil)
	mockStore.EXPECT().GetCredentialsByEmail(gomock.Any(), email).Return(store.EmailAuth{
		AuthID:         authID,
		HashedPassword: hashedPassword,
	}, nil)

	_, err := processor.Login(ctx, email, "wrongpassword")

	if !errors.Is(err, ErrIncorrectPassword) {
		t.Errorf("expected ErrIncorrectPassword, got %v", err)
	}
}

func TestGetUserByExternalID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	userID := uuid.New()
	firstName := "John"
	lastName := "Doe"

	mockStore.EXPECT().GetUserByExternalID(gomock.Any(), userID).Return(store.User{
		ID:        userID,
		FirstName: firstName,
		LastName:  lastName,
	}, nil)

	result, err := processor.GetUserByExternalID(ctx, userID)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.ExternalID != userID {
		t.Errorf("expected ID %s, got %s", userID, result.ExternalID)
	}
	if result.FirstName != firstName {
		t.Errorf("expected firstName %s, got %s", firstName, result.FirstName)
	}
}

func TestGetUserByExternalID_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockAuthStore(ctrl)
	mockBilling := NewMockBillingProcessor(ctrl)
	mockEmail := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, AuthConfig{}, nil, mockBilling, mockEmail, logger)

	ctx := context.Background()
	userID := uuid.New()

	mockStore.EXPECT().GetUserByExternalID(gomock.Any(), userID).Return(store.User{}, store.ErrNotFound)

	_, err := processor.GetUserByExternalID(ctx, userID)

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}
