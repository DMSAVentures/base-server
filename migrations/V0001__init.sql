-- Users Table
CREATE TABLE Users
(
    id          UUID PRIMARY KEY,
    first_name  VARCHAR(100),
    last_name   VARCHAR(100),
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMP DEFAULT NULL
);

-- User Authentication Table with BIGINT
CREATE TABLE User_Auth
(
    id          UUID PRIMARY KEY,
    user_id     UUID REFERENCES Users (id), -- Reference to Users.id
    auth_type  VARCHAR(50), -- Types: 'email', 'google', 'saml'
    last_login TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMP DEFAULT NULL
);

-- Email Authentication Table with BIGINT
CREATE TABLE Email_Auth
(
    auth_id         UUID PRIMARY KEY REFERENCES User_Auth(id),
    email           VARCHAR(255) UNIQUE,
    hashed_password VARCHAR(255),
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMP DEFAULT NULL
);

-- Google Authentication Table with BIGINT
CREATE TABLE Oauth_Auth
(
    auth_id         UUID PRIMARY KEY REFERENCES User_Auth(id),
    external_id   VARCHAR(255),
    email         VARCHAR(255),
    full_name     VARCHAR(255),       -- Added field for Google OAuth
    auth_provider VARCHAR(50),       -- Types: 'google', 'apple', 'facebook'
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMP DEFAULT NULL
);