FROM golang:1.22-alpine AS builder
COPY . /src
WORKDIR /src

RUN go mod download
RUN go build -o /bin/aliassrv /src/cmd/main.go

FROM scratch
EXPOSE 8080
COPY --from=builder /bin/aliassrv /bin/aliassrv
COPY --from=builder /src/config/config.yaml /etc/alias/config.yaml
CMD ["/bin/aliassrv"]