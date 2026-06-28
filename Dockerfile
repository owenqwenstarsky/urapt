# syntax=docker/dockerfile:1

FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/urapt-server ./cmd/urapt-server \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/urapt ./cmd/urapt

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/urapt-server /usr/local/bin/urapt-server
EXPOSE 8080
VOLUME ["/data"]
ENV URAPT_STORE_DIR=/data/store
ENV URAPT_BASE_URL=http://localhost:8080
USER nonroot:nonroot
ENTRYPOINT ["urapt-server"]
CMD ["--bind", "0.0.0.0:8080"]
