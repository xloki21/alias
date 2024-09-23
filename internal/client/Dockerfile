FROM golang:1.22-alpine AS builder
COPY . /src
WORKDIR /src

RUN go mod download
RUN go build -o /bin/alias-client /src/cmd/client/main.go

FROM scratch
EXPOSE 8080
COPY --from=builder /bin/alias-server /bin/alias-server
COPY --from=builder /src/config/config.yaml /etc/alias/config.yaml
CMD ["/bin/alias-server"]