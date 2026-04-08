FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/git-pkg .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/git-pkg /bin/git-pkg

EXPOSE 8080
CMD ["/bin/git-pkg"]
