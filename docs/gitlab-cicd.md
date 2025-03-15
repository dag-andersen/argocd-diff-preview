# GitLab CI/CD Workflow

## Public repositories

If your repository is public and only uses public Helm charts, you can use the following GitLab CI/CD pipeline to generate a diff between the main branch and the merge request branch. The diff will then be posted as a comment on the merge request.

```yaml
default:
  tags:
    - gitlab-org-docker

stages:
  - diff

diff:
  stage: diff
  image: docker:24.0.5
  services:
    - name: docker:24.0.5-dind
  variables:
    GITLAB_TOKEN: $GITLAB_PAT
  before_script:
    - apk add -q curl jq git
  script:
    - |
      echo "******** Running analysis ********"
      git clone ${CI_REPOSITORY_URL} base-branch --depth 1 -q 
      git clone ${CI_REPOSITORY_URL} target-branch --depth 1 -q -b ${CI_MERGE_REQUEST_SOURCE_BRANCH_NAME}
      docker run \
        --network host \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v $(pwd)/output:/output \
        -v $(pwd)/base-branch:/base-branch \
        -v $(pwd)/target-branch:/target-branch \
        -e TARGET_BRANCH=${CI_MERGE_REQUEST_SOURCE_BRANCH_NAME} \
        -e REPO=${CI_MERGE_REQUEST_PROJECT_PATH} \
        dagandersen/argocd-diff-preview:v0.1.0
    - |
      DIFF_BODY=$(jq -Rs '.' < $(pwd)/output/diff.md)
      NOTE_ID=$(curl --silent --header "PRIVATE-TOKEN: ${GITLAB_TOKEN}" \
          "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests/${CI_MERGE_REQUEST_IID}/notes" | \
          jq '.[] | select(.body | test("Argo CD Diff Preview")) | .id')
      
      if [[ -n "$NOTE_ID" ]]; then
          echo "Deleting existing comment (ID: $NOTE_ID)..."

          curl --silent --request DELETE --header "PRIVATE-TOKEN: ${GITLAB_TOKEN}" \
              --url "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests/${CI_MERGE_REQUEST_IID}/notes/${NOTE_ID}"
      fi

      echo "Adding new comment..."
      curl --silent --request POST --header "PRIVATE-TOKEN: ${GITLAB_TOKEN}" \
          --header "Content-Type: application/json" \
          --url "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests/${CI_MERGE_REQUEST_IID}/notes" \
          --data "{\"body\": $DIFF_BODY}" > /dev/null

      echo "Comment added!"
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
```

## Private repositories and Helm Charts

In the simple code example above, we do not provide the cluster with any credentials, which only works if the image/Helm Chart registry and the Git repository are public. Since your repository might not be public, you need to provide the tool with the necessary read-access credentials for the repository. This can be done by placing the Argo CD repo secrets in a folder mounted at /secrets. When the tool starts, it will simply run kubectl apply -f /secrets to apply every resource to the cluster before starting the rendering process.

```yaml
...
  before_script:
    - apk add -q curl jq git
    - |
      mkdir secrets
      cat > secrets/secret.yaml << EOF
      apiVersion: v1
      kind: Secret
      metadata:
        name: private-repo
        namespace: argocd
        labels:
          argocd.argoproj.io/secret-type: repo-creds
      stringData:
        url: https://gitlab.com/${CI_PROJECT_PATH}
        password: ${GITLAB_TOKEN}  ⬅️ Short-lived GitLab Token
        username: token
      EOF

  script:
    - |
      echo "******** Running analysis ********"
      git clone ${CI_REPOSITORY_URL} base-branch --depth 1 -q 
      git clone ${CI_REPOSITORY_URL} target-branch --depth 1 -q -b ${CI_MERGE_REQUEST_SOURCE_BRANCH_NAME}
      docker run \
        --network host \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v $(pwd)/output:/output \
        -v $(pwd)/base-branch:/base-branch \
        -v $(pwd)/target-branch:/target-branch \
        -v $(pwd)/secrets:/secrets \           ⬅️ Mount the secrets folder
        -e TARGET_BRANCH=${CI_MERGE_REQUEST_SOURCE_BRANCH_NAME} \
        -e REPO=${CI_MERGE_REQUEST_PROJECT_PATH} \
        dagandersen/argocd-diff-preview:v0.1.0

```

For more info, see the [Argo CD docs](https://argo-cd.readthedocs.io/en/stable/operator-manual/argocd-repo-creds-yaml/)
