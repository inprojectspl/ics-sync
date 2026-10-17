FROM golang:1.26-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/ics-sync ./cmd/ics-sync

# Alpine rather than scratch: GitLab CI runs the job `script:` with the
# image's shell. The image keeps the root user because the runner writes
# File-type variables (the private key) into the builds directory as the
# job user.
FROM alpine:3.23
# The CA bundle comes from the builder, so the runtime stage needs no
# `apk add` and no access to an Alpine mirror during the build.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/ics-sync /usr/local/bin/ics-sync
ENTRYPOINT ["/usr/local/bin/ics-sync"]
