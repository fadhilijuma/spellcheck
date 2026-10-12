# Go spelling API only, running as non-root; frontend has its own image.
# Build: podman build -t spellcheck .

ARG GO_VERSION=1.27.2

FROM docker.io/library/golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY README.md ./
ARG BUILD=container
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.tag=${BUILD}" -o /out/spellcheck ./cmd/spellcheck
# The licence of every module compiled into the binary, from the module cache.
RUN mkdir -p /out/licenses && \
    go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Dir}}{{end}}{{end}}' ./cmd/spellcheck | sort -u | \
    while read -r path dir; do \
        cp "$(ls "$dir"/LICEN[CS]E* | head -n 1)" "/out/licenses/$(echo "$path" | tr / _).txt" || exit 1; \
    done

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/spellcheck /spellcheck
COPY --from=build /out/licenses /licenses
COPY --from=build /src/README.md /licenses/attribution.txt
EXPOSE 3000
USER nonroot:nonroot
ENTRYPOINT ["/spellcheck"]
