FROM golang:alpine AS builder

WORKDIR /build

COPY go.mod go.sum main.go ./

COPY pkg ./pkg

COPY templates ./templates

RUN go mod download golang.org/x/time

RUN go mod tidy

RUN go build -o docker-cycler main.go


FROM alpine

WORKDIR /build

COPY --from=builder /build/docker-cycler /build/docker-cycler

RUN chmod +x ./docker-cycler

CMD ["./docker-cycler"]