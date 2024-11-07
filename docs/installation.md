# Running it locally

=== "Docker"

    ## Pre-requisites

    - Install: [Docker](https://docs.docker.com/get-docker/)

    ## Usage

    You need to pull down the two branches you want to compare. The first branch will be cloned into the `base-branch` folder, and the other branch will be cloned into the `target-branch` folder.

    ```bash
    git clone https://github.com/<owner>/<repo> base-branch --depth 1 -q -b <branch-a>

    git clone https://github.com/<owner>/<repo> target-branch --depth 1 -q -b <branch-b>
    ```

    Then you can run the tool using the following command:

    ```bash
    docker run \
      --network host \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -v $(pwd)/output:/output \
      -v $(pwd)/base-branch:/base-branch \
      -v $(pwd)/target-branch:/target-branch \
      -e TARGET_BRANCH=<branch-a> \
      -e BASE_BRANCH=<branch-b> \
      -e REPO=<owner>/<repo>  \
      dagandersen/argocd-diff-preview:v0.0.24
    ```

    If base-branch(`BASE_BRANCH`) is not specified it will default to `main`.

=== "Binary"

    ## Pre-requisites

    Install:

      - [Git](https://git-scm.com/downloads)
      - [Docker](https://docs.docker.com/get-docker/)
      - [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
      - [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) OR [minikube](https://minikube.sigs.k8s.io/docs/start/)
      - [Argo CD CLI](https://argo-cd.readthedocs.io/en/stable/cli_installation/)

    ## Find the correct binary for your operating system

    Check the [releases](https://github.com/dag-andersen/argocd-diff-preview/releases) and find the correct binary for your operating system.

    *Example for downloading and running on macOS:*

    ```bash
    curl -LJO https://github.com/dag-andersen/argocd-diff-preview/releases/download/v0.0.24/argocd-diff-preview-Darwin-x86_64.tar.gz
    tar -xvf argocd-diff-preview-Darwin-x86_64.tar.gz
    sudo mv argocd-diff-preview /usr/local/bin
    argocd-diff-preview --help
    ```

    ## Usage

    You need to pull down the two branches you want to compare. The first branch will be cloned into the `base-branch` folder, and the other branch will be cloned into the `target-branch` folder.

    ```bash
    git clone https://github.com/<owner>/<repo> base-branch --depth 1 -q -b <branch-a>

    git clone https://github.com/<owner>/<repo> target-branch --depth 1 -q -b <branch-b>
    ```

    ## Run the binary
    ```bash

    argocd-diff-preview \
      --repo <owner>/<repo-name> \
      --base-branch <branch-a> \
      --target-branch <branch-b>
    ```

    If base-branch is not specified it will default to `main`.

=== "Source"

    ## Pre-requisites

    Install:

    - [Git](https://git-scm.com/downloads)
    - [Docker](https://docs.docker.com/get-docker/)
    - [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
    - [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) OR [minikube](https://minikube.sigs.k8s.io/docs/start/)
    - [Argo CD CLI](https://argo-cd.readthedocs.io/en/stable/cli_installation/)
    - [Rust](https://www.rust-lang.org/tools/install)

    ## Clone the repository

    ```bash
    git clone https://github.com/dag-andersen/argocd-diff-preview
    cd argocd-diff-preview
    cargo run -- --help
    ```

    ## Usage

    You need to pull down the two branches you want to compare. The first branch will be cloned into the `base-branch` folder, and the other branch will be cloned into the `target-branch` folder.

    ```bash
    git clone https://github.com/<owner>/<repo> base-branch --depth 1 -q -b <branch-a>

    git clone https://github.com/<owner>/<repo> target-branch --depth 1 -q -b <branch-b>
    ```

    ## Run the code

    ```bash
    cargo run -- \
      --repo <owner>/<repo-name> \
      --base-branch <branch-a> \
      --target-branch <branch-b>
    ```

    If base-branch is not specified it will default to `main`.

