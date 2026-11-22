# JobyUK.com

Next-generation, high-performance UK job discovery and recruitment platform built in Go, PostgreSQL, and Redis.

## Features

- **Blazing Fast Performance**: Zero-overhead Go backend with Chi routing and server-side rendering.
- **Modern UI & UX**: Pixel-matched design system with lightweight CSS and Turbo navigation.
- **SEO & Indexing**:
  - Real-time Bing & Yahoo IndexNow protocol integration.
  - Automated Google Indexing API daily worker with polite batching.
  - Full structured data (Schema.org / JSON-LD) and dynamic sitemaps.
  - Native AI bot and markdown negotiation (`llms-full.txt`).
- **Comprehensive Taxonomy**: Job roles, industries, cities/regions, salaries, and company directories.
- **Direct Employer Connections**: Apply directly to hiring companies with no middlemen.

## Getting Started

### Prerequisites

- Go 1.22+
- PostgreSQL 15+
- Redis 6+

### Environment Configuration

Copy `.env.example` to `.env` and set your credentials:

```bash
cp .env.example .env
```

### Running Locally

```powershell
.\start_server.ps1
```

Or manually:

```bash
go build -o jobyuk.exe ./cmd/server/main.go
./jobyuk.exe
```

The server runs on `http://localhost:8082` by default.
