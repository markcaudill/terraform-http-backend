FROM golang:1-alpine AS builder
ENV GOPATH /go
ENV PATH $GOPATH/bin:$PATH
COPY . $GOPATH/src/github.com/markcaudill/terraform-http-backend
WORKDIR $GOPATH/src/github.com/markcaudill/terraform-http-backend
RUN set -x && \
	apk add --no-cache --virtual .build-deps gcc libc-dev
RUN go build


FROM alpine:3
LABEL maintainer="Mark Caudill <mark@mrkc.me>"
ENV GOPATH /go
ENV PATH $GOPATH/bin:$PATH
ENV IP 0.0.0.0
COPY --from=builder \
	$GOPATH/src/github.com/markcaudill/terraform-http-backend/terraform-http-backend \
	$GOPATH/bin/terraform-http-backend
CMD ["terraform-http-backend"]
