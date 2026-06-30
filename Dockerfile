# syntax=docker/dockerfile:1

FROM golang:1.26.1-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN mkdir -p /out/data/store \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/urapt-server ./cmd/urapt-server \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/urapt ./cmd/urapt

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/urapt-server /usr/local/bin/urapt-server
COPY --from=build --chown=nonroot:nonroot /out/data /data
EXPOSE 8080
ENV URAPT_STORE_DIR=/data/store
ENV URAPT_BASE_URL=http://localhost:8080
USER nonroot:nonroot
ENTRYPOINT ["urapt-server"]
CMD ["--bind", "0.0.0.0:8080"]
