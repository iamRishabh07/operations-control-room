---
name: go-service
description: Implement or review a small Go HTTP API with resilient live external API calls and focused tests. Use when editing backend services.
---

1. Identify contracts and expected failures; preserve compatibility.
2. Prefer net/http, small handlers, dependency injection for API clients/clock, and `database/sql`.
3. Apply context deadlines, explicit HTTP client timeout, response-body close, status/body validation, JSON decode limits, and structured errors.
4. Use SQL placeholders and avoid leaking secrets.
5. Add narrow tests for recommendation logic and external API failure.
6. Run gofmt, go test ./..., go vet ./... and report actual results.
