### Build stage
FROM golang:1.16-alpine AS go-build

ENV SRC_DIR=/go/src/github.com/eduspire/automated_rds_restore

WORKDIR $SRC_DIR

COPY ./go.mod ./go.mod
COPY ./go.sum ./go.sum

RUN go get -d -v ./...

COPY ./main.go ./main.go
# Run Unit tests
#RUN CGO_ENABLED=0 go test -v test/tests.go

# Build binary
RUN go build -o bin/automated_rds_restore main.go 
# RUN go install -v ./...

### Run stage
FROM alpine:latest

ENV SRC_DIR=/go/src/github.com/eduspire/automated_rds_restore

WORKDIR /usr/local/bin

COPY --from=go-build ${SRC_DIR}/bin/automated_rds_restore .

CMD [ "automated_rds_restore" ]
