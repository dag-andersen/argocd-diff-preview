# Build stage
FROM golang:1-bookworm AS build

# Build arguments for version information
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# https://docs.docker.com/reference/dockerfile/#automatic-platform-args-in-the-global-scope
ARG TARGETARCH

# create a new empty shell project
WORKDIR /argocd-diff-preview

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code - only what's needed
COPY cmd/ ./cmd/
COPY pkg/ ./pkg/

# Build the application with version information
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" \
    -trimpath \
    -o argocd-diff-preview ./cmd

# install kind
RUN apt-get update && apt-get install -y curl
RUN curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.26.0/kind-linux-${TARGETARCH} && \
    chmod +x ./kind

# Install Argo CD
RUN curl -sSL -o argocd-linux-${TARGETARCH} https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-${TARGETARCH} && \
    install -m 555 argocd-linux-${TARGETARCH} /usr/local/bin/argocd && \
    rm argocd-linux-${TARGETARCH}

FROM gcr.io/distroless/static-debian12 AS final

# Copy necessary binaries from the build stage
COPY --from=build /argocd-diff-preview/kind /usr/local/bin/kind
COPY --from=build /usr/local/bin/argocd /usr/local/bin/argocd
COPY --from=build /argocd-diff-preview/argocd-diff-preview .

# Copy docker from the docker image
COPY --from=docker:dind /usr/local/bin/docker /usr/local/bin/

# copy argocd helm chart values
COPY ./argocd-config ./argocd-config

# set the startup command to run your binary
ENTRYPOINT ["./argocd-diff-preview"]