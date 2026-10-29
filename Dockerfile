# eshu-api pre-cutover image: skeleton surface only, no core dispatch yet.
#
# Both stages pin by digest (verified 2026-10-29; re-verify with a registry
# lookup when bumping the tag). The build stage is the multi-arch
# golang:1.26.10-bookworm list; the runtime stage is the multi-arch
# distroless static nonroot OCI index.
FROM golang:1.26.10-bookworm@sha256:23ce71337b726608afb1bc00986e630a05034a87a87ea1fc2cd1ae5cf33387eb AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/eshu-api ./cmd/eshu-api

FROM gcr.io/distroless/static:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99
COPY --from=build /out/eshu-api /eshu-api
EXPOSE 8080
ENTRYPOINT ["/eshu-api"]
