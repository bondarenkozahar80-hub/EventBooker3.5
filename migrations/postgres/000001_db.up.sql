CREATE TABLE events (
                        id SERIAL PRIMARY KEY,
                        name VARCHAR(255) NOT NULL,
                        description TEXT,
                        start_time TIMESTAMP NOT NULL,
                        end_time TIMESTAMP,
                        location VARCHAR(255),

                        capacity INT NOT NULL CHECK (capacity > 0),
                        payment_timeout_minutes INT DEFAULT 15 CHECK (payment_timeout_minutes > 0),

                        created_at TIMESTAMP DEFAULT NOW(),
                        updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE registrations (
                               id SERIAL PRIMARY KEY,
                               event_id INT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
                               full_name VARCHAR(255) NOT NULL,
                               email VARCHAR(255),
                               phone VARCHAR(20),
                               status VARCHAR(50) DEFAULT 'pending',
                               created_at TIMESTAMP DEFAULT NOW(),
                               updated_at TIMESTAMP DEFAULT NOW()
);
