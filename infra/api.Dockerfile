# syntax=docker/dockerfile:1
# Go API — static binary on a distroless base, runs as non-root.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY apps/api ./apps/api
COPY internal ./internal
COPY db ./db
COPY packages/openapi ./packages/openapi
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./apps/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
CMD ["serve"]
