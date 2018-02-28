# docker run --publish 9090:9090 --name echo-server --rm echo-server

FROM golang
ENV GOBIN $GOPATH/bin
ADD . /go/src/github.com/stanhope/echo-server
RUN go install github.com/stanhope/echo-server

ENTRYPOINT /go/bin/echo-server

# Serving HTTP on 9090
EXPOSE 9090
