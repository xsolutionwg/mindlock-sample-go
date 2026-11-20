FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod main.go head.html ./
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -ldflags=-s -o /app .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /app /app
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
