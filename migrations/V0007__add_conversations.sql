CREATE TABLE Conversations
(
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID REFERENCES Users (id),
    title      TEXT,
    created_at TIMESTAMPTZ      DEFAULT NOW(),
    updated_at TIMESTAMPTZ      DEFAULT NOW()
);

CREATE TABLE Messages
(
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID REFERENCES conversations (id),
    role            TEXT CHECK (role IN ('user', 'assistant')) NOT NULL,
    content         TEXT                                       NOT NULL,
    token_count     INT                                        NOT NULL,
    created_at      TIMESTAMPTZ      DEFAULT NOW()
);


CREATE TABLE Usage_Logs
(
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID REFERENCES Users (id),
    conversation_id UUID REFERENCES Conversations (id),
    message_id      UUID REFERENCES Messages (id),
    tokens_used     INT  NOT NULL,
    cost_in_cents   INT  NOT NULL,
    model           TEXT NOT NULL,
    created_at      TIMESTAMPTZ      DEFAULT NOW()
);