CREATE TABLE role (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

INSERT INTO role(name) VALUES ('moderator', 'admin');

CREATE TABLE channel_role (
    id SERIAL PRIMARY KEY,
    channel_name TEXT NOT NULL,
    user_id UUID REFERENCES user(user_id) NOT NULL,
    role INT REFERENCES role(id) NOT NULL
);

CREATE TABLE user (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL DEFAULT gen_random_uuid(),
    username TEXT NOT NULL
);

INSERT INTO user(username) VALUES 'kasutaja123';
