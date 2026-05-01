# Multi-stage build: pin the toolchain in the builder stage, ship a
# distroless runtime so the deployed image has no shell or package
# manager. Mirrors the pattern Arzana's own backend image follows.

FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/hello-world .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/hello-world /hello-world
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/hello-world"]
