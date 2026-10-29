# eshu-api pre-cutover image: skeleton surface only, no core dispatch yet.
FROM golang:1.26.10-bookworm AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/eshu-api ./cmd/eshu-api

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/eshu-api /eshu-api
EXPOSE 8080
ENTRYPOINT ["/eshu-api"]
