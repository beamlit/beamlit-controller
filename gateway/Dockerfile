FROM golang:1.23 AS build

WORKDIR /go/src/app
COPY . .

RUN go mod download
RUN go vet -v
RUN go test -v

RUN CGO_ENABLED=0 go build -o /go/bin/beamlit-proxy 

FROM gcr.io/distroless/static-debian12

COPY --from=build /go/bin/beamlit-proxy /
CMD ["/beamlit-proxy"]