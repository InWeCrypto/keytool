FROM golang:1.9.2-stretch

LABEL maintainer="support@inwecrypto.com"

COPY . /go/src/github.com/inwecrypto/tokenswap

RUN go install github.com/inwecrypto/tokenswap/cmd/tokenswap && rm -rf /go/src

VOLUME ["/etc/inwecrypto/tokenswap"]

WORKDIR /etc/inwecrypto/tokenswap

EXPOSE 8000

CMD ["/go/bin/tokenswap","--conf","/etc/inwecrypto/tokenswap/tokenswap.json"]