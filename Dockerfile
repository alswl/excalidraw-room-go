# Build stage
FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/excalidraw-room-server .

# Runtime stage
FROM alpine:3.20

WORKDIR /app
COPY --from=build /out/excalidraw-room-server .
COPY public ./public

EXPOSE 8080
CMD ["/app/excalidraw-room-server"]
