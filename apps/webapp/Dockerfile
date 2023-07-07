
FROM golang:1.20 AS build-stage

WORKDIR /app

COPY . ./
RUN go get
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/webapp

# Deploy the application binary into a lean image
FROM gcr.io/distroless/base-debian11 AS release-stage

WORKDIR /

COPY --from=build-stage /bin/webapp /webapp
COPY ./templates ./templates
EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/webapp"]