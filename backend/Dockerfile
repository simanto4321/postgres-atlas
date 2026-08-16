# Build context: repository root.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /atlas ./cmd/atlas

FROM gcr.io/distroless/static-debian12
COPY --from=build /atlas /atlas
COPY docs/sample-report.json /sample-report.json
EXPOSE 8000
# Default to demo mode; override ATLAS_DSN for live introspection.
ENV ATLAS_SNAPSHOT=/sample-report.json
ENTRYPOINT ["/atlas"]
