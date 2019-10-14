FROM golang:1.13.1-buster AS builder
ENV GO111MODULE on
WORKDIR /go/src/github.com/form3tech-oss/aws-auth-refresher
COPY go.mod go.sum ./
RUN go mod vendor
COPY main.go .
RUN go build -o /aws-auth-refresher -v ./main.go

FROM gcr.io/distroless/base
USER nobody:nobody
WORKDIR /
COPY --from=builder /aws-auth-refresher /aws-auth-refresher
ENTRYPOINT ["/aws-auth-refresher"]
