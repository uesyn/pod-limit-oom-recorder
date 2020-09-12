FROM golang:1.15.2 as builder
WORKDIR /app
COPY . /app
RUN go build -o pod-limit-oom-recorder

FROM debian:10.5-slim
COPY --from=builder /app/pod-limit-oom-recorder /bin/pod-limit-oom-recorder
ENTRYPOINT ["/bin/pod-limit-oom-recorder"]
