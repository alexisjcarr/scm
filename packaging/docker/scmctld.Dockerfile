FROM golang:1.23-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/scmctld ./cmd/scmctld

FROM debian:bookworm-slim
RUN groupadd --system scmctld && useradd --system --gid scmctld --home /var/lib/scm --shell /usr/sbin/nologin scmctld
RUN install -d -o scmctld -g scmctld /var/lib/scm /etc/scm

COPY --from=build /out/scmctld /usr/local/bin/scmctld

USER scmctld:scmctld
WORKDIR /var/lib/scm

EXPOSE 8443 8080
