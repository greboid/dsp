FROM index.docker.io/golang:1.21 as builder

WORKDIR /app
COPY go.mod go.sum /app/
RUN go mod download
COPY . /app
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -trimpath -ldflags=-buildid= -o main ./cmd

FROM ghcr.io/greboid/dockerfiles/baseroot:latest

COPY --from=builder /app/main /dsp
CMD ["/dsp"]
