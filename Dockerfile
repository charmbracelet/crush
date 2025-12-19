# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOEXPERIMENT=greenteagc go build -ldflags="-s -w" -o karigor .

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY --from=builder /app/karigor .
COPY --from=builder /app/manpages ./manpages
COPY --from=builder /app/completions ./completions

# Set up completion
ENV SHELL=/bin/bash
RUN echo 'source /root/completions/karigor.bash' >> /root/.bashrc

EXPOSE 8080

CMD ["./karigor"]