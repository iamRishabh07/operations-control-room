# One image that runs the same server as `make run`.
#   docker build -t operations-control-room .
#   docker run --rm -p 8080:8080 operations-control-room

FROM node:22-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# modernc.org/sqlite is pure Go, so no C toolchain is needed.
FROM golang:1.25-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY backend/ backend/
COPY data/seed.json data/
RUN CGO_ENABLED=0 go build -o /out/server ./backend/cmd/server
# Build the database here, as `make setup` does, so a small host does not have
# to load 51,744 rows on every start.
RUN mkdir -p /out/data && cp data/seed.json /out/data/ \
 && /out/server -init-only -seed /out/data/seed.json -db /out/data/control.db

FROM alpine:3.20
WORKDIR /app
COPY --from=server /out/server ./server
COPY --from=server --chown=nobody:nobody /out/data ./data
COPY --from=web /src/frontend/dist ./frontend/dist
# Hosts such as Render pass the port in PORT; locally it is 8080.
ENV PORT=8080
EXPOSE 8080
USER nobody
CMD ["sh", "-c", "exec ./server -addr 0.0.0.0:${PORT} -seed data/seed.json -db data/control.db -static frontend/dist"]
