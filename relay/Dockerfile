FROM golang:alpine

WORKDIR /go/src/github.com/textileio/textile-go/relay
COPY . .

RUN apk add --update build-base

RUN go install -v ./...

CMD ["relay"]
