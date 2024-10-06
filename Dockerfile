FROM golang:1.22-alpine AS builder
COPY .. /src
WORKDIR /src

RUN go mod download
RUN go build -o /bin/alias-server /src/cmd/alias/server/main.go
RUN go build -o /bin/service-manager /src/cmd/manager/main.go
RUN go build -o /bin/service-statscollector /src/cmd/stats/main.go


FROM scratch
COPY --from=builder /bin/alias-server /bin/alias-server
COPY --from=builder /bin/service-manager /bin/service-manager
COPY --from=builder /bin/service-statscollector /bin/service-statscollector

EXPOSE 8080


COPY --from=builder /src/config/config.yaml /etc/alias/config.yaml
CMD ["/bin/alias-server"]