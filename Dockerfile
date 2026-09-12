# syntax=docker/dockerfile:1

# Three stages, no shared base: frontend and backend build in parallel-
# capable stages, then only the final static binary (no source, no Node,
# no Go toolchain, no shell) lands in the runtime image. Base image
# digests are pinned (docs/spec/06-container-and-kubernetes-deployment.md
# "Pin base image digests") - see docs/operations.md for how to update
# them: re-pull the tag, take the new digest, edit the three ARGs below.
ARG NODE_IMAGE=node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32
ARG GO_IMAGE=golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

FROM ${NODE_IMAGE} AS frontend
WORKDIR /src/web
# Dependencies first so editing app code doesn't invalidate npm ci's cache
# layer - this is a readability/speed win, not a correctness requirement.
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM ${GO_IMAGE} AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/embed.go
# web/dist is gitignored (generated) - internal/webassets's go:embed needs
# real built files here, not the tracked .gitkeep placeholder, before the
# binary below is compiled.
COPY --from=frontend /src/web/dist/ web/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tenantdeck ./cmd/tenantdeck

FROM ${RUNTIME_IMAGE}
# RUNTIME_IMAGE's :nonroot tag already runs as UID/GID 65532 with no
# shell/package manager and a bundled CA certificate store - exactly what
# docs/spec/06-container-and-kubernetes-deployment.md requires, so there's
# nothing left to configure here.
COPY --from=backend /out/tenantdeck /tenantdeck
EXPOSE 8080
ENTRYPOINT ["/tenantdeck"]
