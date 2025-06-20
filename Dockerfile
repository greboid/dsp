FROM golang:1.24 AS builder

WORKDIR /app
COPY go.mod go.sum /app/
RUN go mod download
COPY . /app
RUN CGO_ENABLED=0 GOOS=linux go build -tags netgo,opusergo -a -trimpath -ldflags='-w -extldflags "-static" -buildid=' -o main ./cmd

FROM ghcr.io/greboid/dockerbase/nonroot:1.20250326.0

COPY --from=builder /app/main /dsp
CMD ["/dsp"]
