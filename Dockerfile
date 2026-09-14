# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY src/ ./src/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/smtprelay ./src

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/smtprelay /smtprelay
USER nonroot:nonroot
EXPOSE 587 465 8080
ENTRYPOINT ["/smtprelay"]
