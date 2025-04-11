FROM golang:1.23-bullseye

# Install cross-compilation tools
RUN apt-get update && apt-get install -y \
    gcc-x86-64-linux-gnu \
    libc6-dev-amd64-cross \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .

# Use the proper cross-compiler
ENV CC=x86_64-linux-gnu-gcc
ENV CGO_ENABLED=1 
ENV GOOS=linux 
ENV GOARCH=amd64

RUN go build -o tonapp ./cmd/api