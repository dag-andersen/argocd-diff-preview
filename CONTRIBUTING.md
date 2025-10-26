# Contributing to ArgoCD Diff Preview

Thank you for your interest in contributing to ArgoCD Diff Preview! This document provides guidelines and instructions for contributing to the project.

## Development Environment Setup

### Prerequisites

To develop for ArgoCD Diff Preview, you'll need the following tools:

1. **Go** (version 1.21 or later) - The main programming language used in the project
2. **Docker** - For building containers and running integration tests
3. **Git** - For version control
4. **Make** - For running the project's build scripts

Additionally, these tools are used by the tool but don't need to be installed directly if you are running the tool with Docker. If you are running the tool with Go, you will need to install them.

- [kind](https://kind.sigs.k8s.io/) - For creating a local Kubernetes cluster
- [kubectl](https://kubernetes.io/docs/reference/kubectl/) - For interacting with the Kubernetes cluster
- [Helm](https://helm.sh/) - For installing Argo CD
- [Argo CD CLI](https://argo-cd.readthedocs.io/en/stable/cli_installation/) - For interacting with Argo CD

### Setting Up Your Development Environment

1. Clone the repository:
   ```bash
   git clone https://github.com/dag-andersen/argocd-diff-preview.git
   cd argocd-diff-preview
   ```

2. Install Go dependencies:
   ```bash
   go mod download
   ```

3. (Optional) Setup for documentation development:
   ```bash
   python3 -m venv venv
   source venv/bin/activate
   pip3 install mkdocs-material
   ```

## Project Structure

```
argocd-diff-preview/
├── cmd/                  # Main application entry points
├── pkg/                  # Core application logic
├── tests/                # Integration tests
├── docs/                 # Documentation
├── argocd-config/        # Argo CD configuraiton that is installed with Argo CD
└── examples/             # Examples used by the integration tests and pull request examples
```

## Building the Project

### Building the Go Binary

```bash
make go-build
```

This will create a binary in the `bin/` directory.

### Building the Docker Image

```bash
make docker-build
```

## Running the Project Locally

There are two ways to run the project locally:

### Using branches from the ArgoCD Diff Preview repository

```bash
make run-with-go target_branch=<your-branch-name>
```
or 
```bash
make run-with-docker target_branch=<your-branch-name>
```

_example to make sure the tool works run:_
```bash
make run-with-go target_branch=helm-example-3
```

### Using branches from your own fork

```bash
make run-with-go target_branch=<your-branch-name> github_org=<your-username>
```

```bash
make run-with-docker target_branch=<your-branch-name> github_org=<your-username>
```

## Testing

ArgoCD Diff Preview uses integration tests to verify functionality. These tests create ephemeral Kubernetes clusters and test the application against various test scenarios.

### Running All Integration Tests

Using Go:
```bash
make run-integration-tests-go
```

Using Docker:
```bash
make run-integration-tests-docker
```

If you have made changes to the output of the tool, you can update the expected output of the tests by setting the `update_expected` variable to `true`. Example:

```bash
make run-integration-tests-docker update_expected=true
```

### Running Unit Tests

To run all unit tests in the project:

```bash
make run-unit-tests
```

## Documentation

The project uses MkDocs for documentation. To serve the documentation locally:

```bash
make mkdocs
```

This will open the documentation in your default browser.

## License

By contributing to ArgoCD Diff Preview, you agree that your contributions will be licensed under the project's license (refer to the LICENSE file in the repository).
