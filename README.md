# Chirpy

Chirpy is a micro-blogging RESTful API server written in Go. It serves as a backend platform where users can register, authenticate, and post short messages called "Chirps".

## Features

* **User Authentication:** Supports user registration and secure login functionality using hashed passwords.
* **Session Management:** Utilizes JWT (JSON Web Tokens) for authentication and supports refresh token generation and revocation.
* **Chirp Management:** Users can create, retrieve, and delete their own chirps.
* **Profanity Filter:** Automatically filters out specific restricted words (e.g., "kerfuffle", "sharbert", "fornax") from user chirps.
* **Length Validation:** Restricts chirps to a maximum of 140 characters.
* **Sorting and Filtering:** Chirps can be sorted by creation date (ascending or descending) and filtered by the author's ID.
* **Webhooks Integration:** Supports third-party webhook events (like Polka) to upgrade a user's account to "Chirpy Red" status.
* **Metrics & Admin:** Tracks and displays the number of visits to the file server, with a dev-only reset capability.

## Tech Stack

* **Language:** Go 1.24.4
* **Database:** PostgreSQL (`lib/pq`)
* **Authentication:** JWT (`golang-jwt/jwt/v5`) and Argon2 / Crypto for password hashing
* **Environment Management:** `joho/godotenv`

## Configuration

To run this application, you must provide a `.env` file in the root directory with the following variables:

* `DB_URL`: The PostgreSQL database connection string.
* `PLATFORM`: The deployment environment (Set to `dev` to enable the database reset endpoint).
* `SECRET`: Your secret key for signing JWTs.
* `POLKA_KEY`: The authorization API key for handling Polka webhooks.

## API Endpoints

### App & Administration
* `GET /app/` - Serves static files and tracks metric hits.
* `GET /api/healthz` - Readiness endpoint; returns "OK".
* `GET /admin/metrics` - Displays an HTML page with the total server hit count.
* `POST /admin/reset` - Resets the database and metric hits (only accessible if `PLATFORM=dev`).

### Users & Authentication
* `POST /api/users` - Creates a new user with an email and password.
* `PUT /api/users` - Updates a user's email and password (requires valid JWT).
* `POST /api/login` - Authenticates a user and returns a short-lived JWT along with a long-lived Refresh Token.
* `POST /api/refresh` - Issues a new JWT using a valid Refresh Token.
* `POST /api/revoke` - Revokes an active Refresh Token.

### Chirps
* `POST /api/chirps` - Creates a new chirp (requires valid JWT).
* `GET /api/chirps` - Retrieves chirps. Can use query parameters like `?sort=asc` or `?author_id={uuid}`.
* `GET /api/chirps/{chirpID}` - Retrieves a specific chirp by its UUID.
* `DELETE /api/chirps/{chirpID}` - Deletes a specific chirp (requires JWT and author ownership).

### Webhooks
* `POST /api/polka/webhooks` - Listens for Polka `user.upgraded` events to set a user's `is_chirpy_red` status (requires `POLKA_KEY` authorization).

## Running the Application
Ensure dependencies are downloaded and the environment variables are set. Start the server via:
```bash
go mod tidy
go run main.go
