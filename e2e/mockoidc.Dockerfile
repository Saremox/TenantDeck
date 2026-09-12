# Builds the E2E-only mock OIDC provider (cmd/mockoidc) - never part of
# the production image (see the root Dockerfile, which doesn't reference
# this file or cmd/mockoidc at all). Same base image choices as the
# production build for consistency, not because this needs production
# hardening.
ARG GO_IMAGE=golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

FROM ${GO_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/mockoidc/ cmd/mockoidc/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mockoidc ./cmd/mockoidc

FROM ${RUNTIME_IMAGE}
COPY --from=build /out/mockoidc /mockoidc
EXPOSE 9999
ENTRYPOINT ["/mockoidc"]
