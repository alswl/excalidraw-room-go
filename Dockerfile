# Build stage
FROM golang:1.26-alpine AS build

# Overridable Go module proxy (e.g. a China mirror via --build-arg).
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY

# Version stamped into the binary; set via --build-arg VERSION=<git tag>.
ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/alswl/excalidraw-room-go/pkg/version.Version=${VERSION}" -o /out/excalidraw-room-server ./cmd/server

# Runtime stage
FROM alpine:3.20

WORKDIR /app
COPY --from=build /out/excalidraw-room-server .
COPY public ./public

EXPOSE 8080
CMD ["/app/excalidraw-room-server"]
