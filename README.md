# DouzTach 🚀

### A Modern Team Collaboration & Project Management Platform

DouzTach is a command-line based collaboration platform designed to help teams organize their work, manage projects, track tasks, and communicate efficiently.

Built with **Go**, **PostgreSQL**, and modern backend technologies, DouzTach aims to provide a lightweight, reliable, and developer-friendly experience through a terminal-first interface.

---

## 📌 Project Status

**Currently under active development.**

The project is being developed incrementally, starting with the CLI foundation, backend infrastructure, database integration, and user management.

## ✨ Vision & Features

DouzTach aims to provide:

- **User Authentication** — Secure registration and login.
- **Workspaces** — Create and manage collaborative workspaces.
- **Team Management** — Invite members and manage permissions.
- **Task Management** — Create, assign, and track tasks.
- **Scrum Management** — Organize team workflows and activities.
- **Real-time Communication** — Chat and live notifications.
- **Terminal Interface** — Interact with the platform directly from the command line.

*Features are planned and implemented progressively.*

---

## 🛠️ Tech Stack

| Technology     | Purpose                           |
| -------------- | --------------------------------- |
| Go             | Backend & CLI development         |
| PostgreSQL     | Relational database               |
| Chi            | HTTP routing                      |
| pgx            | PostgreSQL connectivity           |
| sqlc           | Type-safe SQL code generation     |
| Cobra          | CLI command management            |
| Lip Gloss      | Terminal styling                  |
| Docker Compose | Local development environment     |
| WebSocket      | Real-time communication (planned) |

---

## 🏗️ Architecture

```text
DouzTach/
│
├── client/                  # CLI client
│   ├── api/
│   ├── commands/
│   ├── config/
│   ├── terminal/
│   └── websocket/
│
├── cmd/
│   ├── client/              # CLI entry point
│   └── server/              # API entry point
│
├── configs/                 # Application configuration
├── deployments/             # Deployment resources
├── docs/                    # Project documentation
│
├── internal/
│   ├── database/            # Database connection
│   ├── permissions/         # Permission management
│   ├── repository/          # Generated database queries
│   ├── services/
│   └── validator/
│
├── migrations/              # SQL migrations
├── queries/                 # SQL queries for sqlc
│
├── server/
│   ├── activity/
│   ├── api/
│   │   ├── handlers/
│   │   ├── middleware/
│   │   └── routes/
│   ├── auth/
│   ├── chat/
│   ├── invitations/
│   ├── members/
│   ├── notifications/
│   ├── scrum/
│   ├── tasks/
│   ├── users/
│   └── workspaces/
│
├── compose.yaml
├── sqlc.yaml
├── go.mod
└── README.md
```

---

## ⚙️ Getting Started

### Prerequisites

Make sure you have installed:

- [Go](https://go.dev/dl/)
- [Docker](https://www.docker.com/)
- [Git](https://git-scm.com/)

### 1. Clone the repository

```bash
git clone https://github.com/ben-anass/DouzTach.git
cd DouzTach
```

### 2. Configure environment variables

Create your local environment file:

```bash
cp .env.example .env
```

Configure your PostgreSQL credentials in `.env`:

```env
POSTGRES_USER=your_username
POSTGRES_PASSWORD=your_password
POSTGRES_DB=douztach
POSTGRES_PORT=5432
```

**Important:** Never commit your `.env` file or expose your database credentials.

### 3. Start PostgreSQL

```bash
docker compose up -d
```

Check the database status:

```bash
docker compose ps
```

### 4. Run the API server

```bash
go run ./cmd/server
```

The API server will start at:

```text
http://localhost:8080
```

### 5. Run the CLI

In another terminal:

```bash
go run ./cmd/client
```

---

## 🌐 API

The initial API provides basic endpoints:

| Method | Endpoint  | Description          |
| ------ | --------- | -------------------- |
| GET    | `/`       | API welcome response |
| GET    | `/health` | API health check     |

### Health Check

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{
  "status": "ok",
  "app": "DouzTach"
}
```

---

## 🗄️ Database

DouzTach uses PostgreSQL as its primary database.

Versioned schema changes live in `migrations/`, with matching up/down SQL files. Query definitions live in `queries/` and are converted into type-safe Go code using sqlc. The Scrum service in `server/scrum/` provides workspace-scoped epic, user story, work-item, backlog, and sprint operations; mutations and their activity records are committed in the same database transaction. Deletes are soft deletes so history is retained. Sprint membership uses `sprint_stories` as its source of truth, and PostgreSQL enforces one active sprint per workspace.

Generate database code:

```bash
sqlc generate
```

Generated files are stored in:

```text
internal/repository/
```

The Scrum service validates work-item hierarchy, status/priority values, date ranges, workspace-scoped references, and assignee membership before persisting changes.

---

## 🧪 Development

Format Go source files:

```bash
gofmt -w .
```

Build the project:

```bash
go build ./...
```

Run Go tests:

```bash
go test ./...
```

---

## 🗺️ Roadmap

- [x] Initial Go project structure
- [x] Command-based CLI foundation
- [x] Basic terminal commands
- [x] PostgreSQL Docker environment
- [x] Database connection using pgx
- [x] Initial HTTP API
- [x] Initial users database migration
- [x] SQL query generation with sqlc
- [ ] Complete user registration
- [ ] Secure login and authentication
- [ ] Workspace management
- [ ] Team invitations and permissions
- [ ] Task management
- [ ] Scrum workflows
- [ ] Real-time chat and notifications
- [ ] VPS deployment with HTTPS

---

## 👥 Team & Responsibilities

DouzTach is developed by a team of three members, with each member responsible for a specific part of the project. We collaborate to build an integrated, reliable, and scalable team collaboration platform.

### 🖥️ Member 1 — `HSassouma`
**CLI & Terminal UI Client**

Responsible for the client-side terminal experience and interface design.

- CLI architecture and command management.
- Terminal UI design and dashboard.
- Interactive prompts and user navigation.
- Tables, colors, and terminal styling.
- Client-side user experience.

### ⚙️ Member 2 — `ben-anass`
**Backend & Authentication**

Responsible for the server-side architecture, API development, and user authentication.

- Go REST API development.
- User registration and login.
- User management.
- Workspaces and member management.
- Invitations system.
- Permissions and access control.

### 🗄️ Member 3 — `0xm3d`
**Database & Collaboration Features**

Responsible for database architecture and core collaboration features.

- PostgreSQL database management.
- Database schema and migrations.
- Tasks and epics management.
- Scrum workflows.
- Real-time chat.
- Notifications system.
- Activity tracking.

---

### 🤝 Collaboration

Each member works on a dedicated development area while collaborating with the rest of the team through Git and a shared development workflow.

Our goal is to combine our individual contributions into one unified platform: **DouzTach**.

**Three members. Three responsibilities. One project.** 🚀


---


## 🌿 Git Workflow

The project uses a three-branch workflow:

| Branch | Purpose                 |
| ------ | ----------------------- |
| `main` | Stable version          |
| `test` | Integration and testing |
| `nani` | Development branch      |

Changes are developed on feature branches, integrated into `test`, and promoted to `main` after verification.

---

## 🎯 Project Goals

DouzTach aims to combine the simplicity of terminal-based workflows with the power of modern collaboration tools.

The long-term goal is to build a reliable, extensible, and efficient platform that helps development teams stay organized and productive without unnecessary complexity.

---

## 👨‍💻 Development

**Repository:** [ben-anass/DouzTach](https://github.com/ben-anass/DouzTach)

**Status:** Active Development

---

\<p align="center">
&#x20; \<strong>DouzTach\</strong>\<br/>
&#x20; \<i>Build. Collaborate. Deliver.\</i>
\</p>
