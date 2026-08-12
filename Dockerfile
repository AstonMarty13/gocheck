# --- build stage -------------------------------------------------------------
FROM golang:1.26-alpine AS build

WORKDIR /src

# ca-certificates and tzdata are copied into the scratch image below; the
# alpine base does not ship them.
RUN apk add --no-cache ca-certificates tzdata

# Cache dependencies separately from sources. gocheck has none today, but this
# keeps rebuilds cheap if that ever changes.
COPY go.mod ./
RUN go mod download

COPY . .

# Fully static binary: no libc, no dynamic loader, so it runs on scratch.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gocheck ./cmd

# --- runtime stage -----------------------------------------------------------
FROM scratch

# gocheck talks HTTPS, so it needs the CA bundle and a timezone database.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=build /out/gocheck /gocheck

# Mount your own list over this path:
#   docker run -v "$PWD/sites.txt:/etc/gocheck/sites.txt:ro" gocheck
COPY sites.txt /etc/gocheck/sites.txt

USER 65534:65534
EXPOSE 8080

ENTRYPOINT ["/gocheck"]
CMD ["-sites", "/etc/gocheck/sites.txt", "-addr", ":8080"]
