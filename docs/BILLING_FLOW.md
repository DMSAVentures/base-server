# Billing Flow Documentation

This document describes the billing system architecture and user flows for LaunchCamp's subscription management.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Stripe Product Structure](#stripe-product-structure)
- [User Flows](#user-flows)
- [API Endpoints](#api-endpoints)
- [Key Design Decisions](#key-design-decisions)

---

## Architecture Overview

```mermaid
graph TB
    subgraph Frontend ["Frontend (webapp)"]
        PP[Plans Page]
        BP[Billing Page]
        CP[Checkout Page]
    end

    subgraph Backend ["Backend (base-server)"]
        BH[Billing Handler]
        BPROC[Billing Processor]
        SS[Subscription Service]
        ST[Store]
    end

    subgraph Stripe ["Stripe"]
        PORTAL[Billing Portal]
        CHECKOUT[Checkout Session]
        SUB[Subscriptions]
        WH[Webhooks]
    end

    subgraph Database ["Database"]
        USERS[(Users)]
        SUBS[(Subscriptions)]
        PRICES[(Prices)]
    end

    PP --> BH
    BP --> BH
    CP --> BH
    BH --> BPROC
    BPROC --> SS
    BPROC --> ST
    SS --> ST
    ST --> Database
    BPROC --> Stripe
    WH --> BH
```

---

## Stripe Product Structure

```mermaid
graph LR
    subgraph Products
        FREE[LaunchCamp Free]
        PRO[LaunchCamp Pro]
        TEAM[LaunchCamp Team]
    end

    subgraph Prices
        FREE --> F1["$0/mo<br/>lookup: free"]
        PRO --> P1["$29/mo<br/>lookup: lc_pro_monthly"]
        PRO --> P2["$290/yr<br/>lookup: lc_pro_annual"]
        TEAM --> T1["$79/mo<br/>lookup: lc_team_monthly"]
        TEAM --> T2["$790/yr<br/>lookup: lc_team_annual"]
    end
```

| Product | Interval | Lookup Key | Price |
|---------|----------|------------|-------|
| LaunchCamp Free | Monthly | `free` | $0/mo |
| LaunchCamp Pro | Monthly | `lc_pro_monthly` | $29/mo |
| LaunchCamp Pro | Annual | `lc_pro_annual` | $290/yr |
| LaunchCamp Team | Monthly | `lc_team_monthly` | $79/mo |
| LaunchCamp Team | Annual | `lc_team_annual` | $790/yr |

---

## User Flows

### 1. New User Signup Flow

```mermaid
sequenceDiagram
    participant U as User
    participant FE as Frontend
    participant BE as Backend
    participant S as Stripe

    U->>FE: Sign up
    FE->>BE: POST /api/auth/signup
    BE->>S: Create Stripe Customer
    S-->>BE: customer_id
    BE->>S: Create $0 Free Subscription
    S-->>BE: subscription_id
    BE->>BE: Store user + subscription
    BE-->>FE: Success
    FE-->>U: Redirect to Dashboard
```

### 2. Upgrade Flow (Free → Paid)

When a user with a free subscription wants to upgrade to Pro or Team:

```mermaid
sequenceDiagram
    participant U as User
    participant FE as Frontend (Plans Page)
    participant BE as Backend
    participant S as Stripe
    participant SP as Stripe Portal

    U->>FE: Click "Pro" plan
    FE->>FE: Check hasActiveSubscription ✓
    FE->>BE: POST /billing/create-customer-portal
    BE->>S: Create Portal Session
    S-->>BE: portal_url
    BE-->>FE: {url: portal_url}
    FE->>SP: Redirect to Portal

    Note over SP: User selects new plan<br/>in portal

    U->>SP: Add payment method
    U->>SP: Select plan & confirm upgrade
    SP->>S: Update subscription
    S->>BE: Webhook: subscription.updated
    BE->>BE: Update local subscription
    SP-->>FE: Redirect to /billing
```

### 3. Plan Change Flow (Paid → Different Paid)

```mermaid
sequenceDiagram
    participant U as User
    participant FE as Frontend
    participant BE as Backend
    participant SP as Stripe Portal

    U->>FE: Click different plan
    FE->>BE: POST /billing/create-customer-portal
    BE-->>FE: {url: portal_url}
    FE->>SP: Redirect to Portal

    Note over SP: Shows plan change with<br/>proration preview

    U->>SP: Select new plan & confirm
    SP-->>FE: Redirect to /billing
```

### 4. Downgrade Flow (Paid → Free)

```mermaid
sequenceDiagram
    participant U as User
    participant FE as Frontend
    participant BE as Backend
    participant SP as Stripe Portal

    U->>FE: Click "Free" plan
    FE->>FE: Check hasActiveSubscription ✓
    FE->>BE: POST /billing/create-customer-portal
    BE-->>FE: {url: portal_url}
    FE->>SP: Redirect to Portal

    Note over SP: Shows downgrade confirmation<br/>Effective at billing period end

    U->>SP: Select Free plan & confirm
    SP-->>FE: Redirect to /billing
```

### 5. New User Selects Paid Plan (No Existing Subscription)

This flow only applies to users who somehow don't have a subscription (edge case):

```mermaid
sequenceDiagram
    participant U as User
    participant FE as Frontend
    participant BE as Backend
    participant S as Stripe

    U->>FE: Click "Pro" plan
    FE->>FE: Check hasActiveSubscription ✗
    FE->>FE: Navigate to /billing/pay?plan=price_id
    FE->>BE: POST /billing/create-checkout-session<br/>{price_id: "lc_pro_monthly"}
    BE->>BE: Check no existing subscription
    BE->>S: Create Checkout Session
    S-->>BE: {client_secret}
    BE-->>FE: {client_secret}
    FE->>FE: Render Stripe Embedded Checkout
    U->>FE: Complete payment
    S->>BE: Webhook: checkout.session.completed
    BE->>BE: Create subscription record
```

### 6. Checkout Session Prevention (Already Has Subscription)

```mermaid
sequenceDiagram
    participant FE as Frontend
    participant BE as Backend

    FE->>BE: POST /billing/create-checkout-session
    BE->>BE: GetActiveSubscription()

    alt Has Active Subscription
        BE-->>FE: 409 Conflict<br/>{code: "SUBSCRIPTION_EXISTS"}
    else No Subscription
        BE->>BE: Continue with checkout...
    end
```

---

## API Endpoints

### Billing Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/billing/plans` | Get all available prices (public) |
| `GET` | `/api/protected/billing/subscription` | Get current user's subscription |
| `POST` | `/api/protected/billing/create-checkout-session` | Create new subscription checkout |
| `POST` | `/api/protected/billing/create-customer-portal` | Get billing portal URL |
| `DELETE` | `/api/protected/billing/cancel-subscription` | Cancel subscription |

### Create Customer Portal Request

No request body required. The endpoint returns a URL to the Stripe Billing Portal where users can manage their subscription.

---

## Key Design Decisions

### 1. Single Subscription Per User

```mermaid
graph TD
    A[User attempts action] --> B{Has subscription?}
    B -->|Yes| C{Action type?}
    B -->|No| D[Allow checkout]
    C -->|Create new| E[409 SUBSCRIPTION_EXISTS]
    C -->|Update/Change| F[Use Billing Portal]
```

- Each user can only have ONE active subscription
- `CreateCheckoutSession` checks for existing subscription and returns 409 if found
- All plan changes go through Stripe Billing Portal

### 2. Billing Portal for All Changes

```mermaid
graph LR
    subgraph "Plan Changes"
        U[Upgrade] --> BP[Billing Portal]
        D[Downgrade] --> BP
        I[Interval Change] --> BP
        PM[Payment Method] --> BP
    end

    subgraph "Stripe Handles"
        BP --> PR[Proration]
        BP --> INV[Invoicing]
        BP --> PM2[Payment Methods]
        BP --> TAX[Tax Calculation]
    end
```

**Why Billing Portal?**
- Stripe handles proration calculations automatically
- Built-in payment method collection
- Secure card handling (PCI compliant)
- Consistent UX across all billing actions
- Less code to maintain

### 3. Portal Configuration

The billing portal uses an explicit configuration ID (`bpc_...`) that has subscription updates enabled. This ensures the correct portal configuration is used regardless of Stripe's default configuration settings.

```go
params := &stripe.BillingPortalSessionParams{
    Customer:      stripe.String(stripeCustomerID),
    ReturnURL:     stripe.String(fmt.Sprintf("%s/billing", webhostURL)),
    Configuration: stripe.String("bpc_..."), // Portal config with subscription updates enabled
}
```

### 4. Free Subscription on Signup

```mermaid
graph TD
    A[User Signs Up] --> B[Create Stripe Customer]
    B --> C[Create $0 Free Subscription]
    C --> D[User has active subscription]
    D --> E[All future changes via Portal]
```

- Every user starts with a $0 subscription
- Simplifies upgrade flow (always update, never create new)
- Consistent subscription state for all users

---

## Stripe Dashboard Configuration

### Customer Portal Settings

Navigate to **Stripe Dashboard → Settings → Billing → Customer portal**:

1. **Subscriptions**
   - [x] Allow customers to switch plans
   - [x] Limit to 1 subscription
   - Add products: Free, Pro, Team

2. **Payment Methods**
   - [x] Allow customers to update payment methods

3. **Invoices**
   - [x] Show invoice history

4. **Cancellations**
   - [x] Allow customers to cancel (optional)

---

## Error Handling

| Error | HTTP Status | Code | Description |
|-------|-------------|------|-------------|
| No subscription found | 404 | - | User has no active subscription |
| Subscription exists | 409 | `SUBSCRIPTION_EXISTS` | Cannot create checkout when subscription exists |
| Failed to get subscription | 500 | - | Internal error fetching subscription |

---

## Webhook Events

The following Stripe webhooks should be configured:

| Event | Handler Action |
|-------|---------------|
| `customer.subscription.created` | Create local subscription record |
| `customer.subscription.updated` | Update local subscription (plan, status) |
| `customer.subscription.deleted` | Mark subscription as cancelled |
| `invoice.paid` | Update billing dates |
| `invoice.payment_failed` | Mark subscription as `past_due` |
