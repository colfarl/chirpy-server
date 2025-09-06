## Chirpy Server 🐦

A lightweight RESTful API server built in **Go**, modeled after a very basic Twitter clone.  
This project was completed as part of the **Boot.dev Backend Development Course** to deepen my technical knowledge in backend systems.

---

## 📌 Why I Built This
I enrolled in the Boot.dev course to sharpen my skills beyond theory and directly **apply backend concepts in Go**.  
One module was dedicated to building an HTTP server, and this is a guided project on a  **mini Twitter-like service** with authentication, authorization, and persistence.

---

## 🚀 What I Learned
### New Topics
- **API Design** → Designing clean RESTful endpoints with conventional routes and status codes.  
- **Authentication & Authorization** → Using **JWTs** for access tokens and refresh token workflows.  
- **Cryptography Basics** → Applying **bcrypt** password hashing and exploring other options.  
- **HTTP Conventions** → Correct use of verbs (`GET`, `POST`, `PUT`, `DELETE`) and response codes.  
- **Building Servers in Go** → Using `net/http`, handlers, middleware, and structured request/response flows.

### Skills I Honed
- **Database Design** → Modeling users, chirps, and refresh tokens in **Postgres**.  
- **Go & SQL Integration** → Leveraging `database/sql` with generated query structs.  
- **Unit Testing & Debugging** → Verifying handler behavior and ensuring safe token flows.  
- **Clean Code Practices** → Using clear separation of concerns, middleware, and structured configs.  
- **Systems Thinking** → Designing authentication flows that align with real-world security patterns.

---

## ⚙️ Functionality
The server provides a full mini social API:

- **Health Check**  
  - `GET /api/healthz` → returns `OK` to confirm the server is alive.

- **User Management**  
  - `POST /api/users` → create a new user (with hashed password).  
  - `POST /api/login` → login with email/password, returns JWT + refresh token.  
  - `PUT /api/users` → update user email/password.  
  - `POST /api/refresh` → issue a new JWT from a refresh token.  
  - `POST /api/revoke` → revoke a refresh token.  
  - `POST /api/polka/webhooks` → handle external webhook events (upgrade user to Chirpy Red).

- **Chirps (Tweets)**  
  - `POST /api/chirps` → create a chirp (max 140 chars, profanity filtered).  
  - `GET /api/chirps` → fetch all chirps, with optional sort (`asc`, `desc`) and filter by author.  
  - `GET /api/chirps/{chirpID}` → fetch a single chirp by ID.  
  - `DELETE /api/chirps/{chirpID}` → delete a chirp (only if owned by user).

- **Admin Utilities**  
  - `GET /admin/metrics` → view total file server hits.  
  - `POST /admin/reset` → reset metrics and clear the database (restricted to `dev` mode).

---

## 🛠️ Tech Stack
- **Language:** Go (1.21+)  
- **Database:** PostgreSQL (15+)  
- **Auth:** JWTs, refresh tokens, bcrypt password hashing  
- **Server:** `net/http` with structured routing and middleware  
- **Other:** Environment configuration with `godotenv`

---

## 📚 Example Skills Demonstrated
- Writing production-style Go code with **structured packages** (`internal/auth`, `internal/database`).  
- Secure login systems with **password hashing** and **token expiration**.  
- Implementing a **refresh token rotation & revocation strategy**.  
- Designing a schema and queries in **Postgres** with Go integration.  
- Building and testing endpoints for **CRUD operations**.

---

## 🔮 Takeaways
This project was a major step in strengthening my backend engineering skills.  
It demonstrates my ability to:
- **Pick up new technologies quickly** (Go + Postgres).  
- **Apply security best practices** in real-world authentication flows.  
- **Build complete systems** that are clean, testable, and extensible.  

I’m excited to continue building projects like this to keep leveling up as a backend/solutions engineer.

---
