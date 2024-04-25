-- Users Table with BIGINT and external_id as UUID
CREATE TABLE Users
(
    id     BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    external_id UUID      DEFAULT gen_random_uuid(), -- Unique identifier for APIs
    first_name  VARCHAR(100),
    last_name   VARCHAR(100),
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- User Authentication Table with BIGINT
CREATE TABLE User_Auth
(
    auth_id    BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    user_id    BIGINT,
    auth_type  VARCHAR(50), -- Types: 'email', 'google', 'saml'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES Users (id)
);

-- Email Authentication Table with BIGINT
CREATE TABLE Email_Auth
(
    auth_id         BIGINT PRIMARY KEY, -- Reference to User_Auth.auth_id
    email           VARCHAR(255) UNIQUE,
    hashed_password VARCHAR(255),
    FOREIGN KEY (auth_id) REFERENCES User_Auth (auth_id)
);

-- Google Authentication Table with BIGINT
CREATE TABLE Oauth_Auth
(
    auth_id       BIGINT PRIMARY KEY, -- Reference to User_Auth.auth_id
    user_id       VARCHAR(255),
    email         VARCHAR(255),
    full_name     VARCHAR(255),       -- Added field for Google OAuth
    auth_provider VARCHAR(50),        -- Types: 'google', 'apple', 'facebook'
    FOREIGN KEY (auth_id) REFERENCES User_Auth (auth_id)
);