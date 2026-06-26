# Contributing to Cluster Policy Controller

This document serves as a guide for contributing to the cluster-policy-controller, which is maintained by the OpenShift Control Plane group.

This document is explicitly for contributions to the cluster-policy-controller repository and not for high-level feature proposals within OpenShift.

Feature proposals should follow the OpenShift Enhancement Proposal process outlined in https://github.com/openshift/enhancements/blob/master/dev-guide/feature-zero-to-hero.md#openshift-feature-development-zero-to-hero-guide.
If you are looking for a review on an OpenShift Enhancement Proposal that involves changes to the cluster-policy-controller, please request a review in the [`#forum-ocp-apiserver`](https://redhat.enterprise.slack.com/archives/CB48XQ4KZ) Slack channel.

## Table of Contents

- [Code Conventions](#code-conventions)
- [Testing Guidelines](#testing-guidelines)
- [Pull Request Process and Guidelines](#pull-request-process-and-guidelines)
- [Review Expectations](#review-expectations)
- [Local Development Guide](#local-development-guide)

---

## Code Conventions

We largely follow the [Kubernetes Code Conventions](https://github.com/kubernetes/community/blob/main/contributors/guide/coding-conventions.md#code-conventions).

Review both the Kubernetes Code Conventions and the ones specified here. If any conventions are at odds with one another, prefer the conventions explicitly documented here.

### Bash

- Follow the [shell styleguide](https://google.github.io/styleguide/shellguide.html).
- Use [`shellcheck`](https://github.com/koalaman/shellcheck) to identify common mistakes or caveats.
- Ensure that all scripts run consistently across Linux and MacOS.

### Golang (Go)

- Review [Effective Go](https://go.dev/doc/effective_go).
- Review common [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).
- Review and avoid [Go Landmines](https://gist.github.com/lavalamp/4bd23295a9f32706a48f)
- Comment your code following the [Go comment conventions](https://go.dev/doc/comment).
    - Comments should be meaningful and add context and/or explain choices that cannot be expressed through clear code.
    - All exported types, functions, and methods must have descriptive comments.
    - All unexported types, functions, and methods should have descriptive comments.
- When adding command-line flags, use dashes/hyphens (`-`) and not underscores (`_`).
- Naming
    - Please consider package name when selecting an interface name, and avoid redundancy. For example, `storage.Interface` is better than `storage.StorageInterface`.
    - Do not use uppercase characters, underscores, or dashes in package names.
    - Please consider parent directory name when choosing a package name. For example, `pkg/controllers/autoscaler/foo.go` should say `package autoscaler` not `package autoscalercontroller`.
        - Unless there's a good reason, the package foo line should match the name of the directory in which the .go file exists.
        - Importers can use a different name if they need to disambiguate.
    - Locks should be called `lock` and should never be embedded (always `lock sync.Mutex`). When multiple locks are present, give each lock a distinct name following Go conventions: `stateLock`, `mapLock` etc.
- Error handling
    - Wrap errors with meaningful context before returning or logging them.
- When logging, follow the [Kubernetes Logging Conventions](https://github.com/kubernetes/community/blob/main/contributors/devel/sig-instrumentation/logging.md).
- When patching OpenShift-maintained forks of "upstream" repositories, patches should be as small as reasonably possible and should minimize touch points with code that is likely to change and impact the rebasing process.

### General

Regardless of the programming language, make sure to take the following into consideration:
- Keep readability / maintainability in mind when writing code.
    - Clever code and abstractions are often harder to reason about after the fact. Keep clever code and abstractions to the minimum necessary to accomplish the end-goal.
- Do not reinvent the wheel. Where possible, use existing standard library or vendored library functionality. If you are adding a net-new dependency, stop and think if you _really_ need to add the new dependency to achieve your goals.
- When writing tests, focus on testing the functional behaviors your code exercises. Avoid writing tests that are testing that the standard library works as expected or is trivial. Do not write tests just for line coverage.

### Directory and File Conventions

- Avoid package sprawl. Find an appropriate subdirectory for new packages.
    - Libraries with no appropriate home belong in new package subdirectories of `pkg/`.
- Avoid general utility packages. Packages called "util" are suspect. Instead, derive a name that describes your desired function.
- All filenames should be lowercase.
- Go source files and directories use underscores, not dashes.
    - Package directories should generally avoid using separators as much as possible. When package names are multiple words, they usually should be in nested subdirectories.

---

## Testing Guidelines

These are high-level testing guidelines. The cluster-policy-controller has additional testing specifics documented in the [Testing Your Changes](#testing-your-changes) section below.

- All changes must include unit test additions/changes.
    - Exceptions are at reviewer/approver discretion.
- Table-driven unit tests are preferred for testing multiple scenarios/inputs. For an example, see existing tests in `pkg/psalabelsyncer/` or `pkg/security/controller/`.
- Unit tests must pass on all platforms (at the very least, Linux + MacOS).
- Significant features should come with integration and/or end-to-end (e2e) tests where appropriate.
    - End-to-end tests _may_ be scoped as a separate work item when the end-to-end tests for the component must be added to the openshift/origin repository instead of the component repository. It is up to reviewer/approver discretion whether a contribution can be merged without end-to-end tests being implemented.
- Do not expect an asynchronous thing to happen immediately. Do not wait for one second and expect a pod to be running. Wait and retry instead.

If necessary, manual integration testing can be done by creating a cluster using the [`Cluster Bot` Slack App](https://redhat.enterprise.slack.com/archives/D03KX7M1CRJ).
Once you have a cluster created, you can follow some of the instructions in https://github.com/openshift/enhancements/blob/master/dev-guide/operators.md for guidance on how to build component images and modify cluster-operators to deploy those images.

Most component repos have existing tooling to run unit tests. For cluster-policy-controller:

```bash
# Run unit tests
make test

# Run all verification
make verify
```

---

## Pull Request Process and Guidelines

This section assumes that you have a functional understanding of `git` and how to create a pull request on GitHub.

If you do not, start with [GitHub's "Getting Started" guide](https://docs.github.com/en/get-started/start-your-journey).

### Prerequisites

Before you commit any changes or create any pull requests, you must adhere to OpenShift contribution policies.
Currently, that means enabling commit signature verification.

See https://docs.google.com/document/d/1184EPSGunUkcSQYUK8T4a6iyawwi6f2zxdbB2jtG9nQ/edit?usp=sharing for more details on how to adhere to the commit signature verification policy of OpenShift.

**To enable commit signing**:

```bash
git config --global user.signingkey <your-gpg-key-id>
git config --global commit.gpgsign true
```

### Creating a Pull Request

When creating a pull request, include the following:

- A brief, but descriptive, title.
    - All pull requests _should_ link to a Jira ticket associated with the work. There is automation that performs this linking when prefixing the title with the Jira ticket identifier like: `OCPBUGS-XXXX: my pull request title`. For pull requests that have no Jira ticket associated with it, you can prefix it with `NO-JIRA:` to signal that there is not a Jira ticket associated with it.
- A useful description of the changes being made and why they are important. Include links to supporting documents and any additional context that reviewers may need.

**Example PR Title**:
```
OCPBUGS-12345: fix UID range allocation for namespaces with existing annotations
```

or

```
NO-JIRA: fix typo in CONTRIBUTING.md
```

### Rebase Checklist

When rebasing to a new Kubernetes version, copy this checklist into the PR:

- [ ] Select the desired [kubernetes release branch](https://github.com/kubernetes/kubernetes/branches), and use its `go.mod` and `CHANGELOG` as references for the rest of the work.
- [ ] Bump go version, all `k8s.io/`, `github.com/openshift/`, and any other relevant dependencies as needed.
- [ ] Run `go mod vendor && go mod tidy`, commit that separately from all other changes.
- [ ] Bump image versions (Dockerfile, ci...) if needed.
- [ ] Run `make build verify test`.
- [ ] Make code changes as needed until the above pass.
- [ ] Any other minor update, like documentation.

### CI / CD

For CI/CD, OpenShift uses Prow to run various checks. This can include unit tests, e2e tests, linters, etc.

The jobs configured for each repository are in https://github.com/openshift/release/tree/main/ci-operator/config/openshift . If you find yourself needing to add additional jobs, review the documentation at https://docs.ci.openshift.org/how-tos/contributing-openshift-release/ .

There are often a mixture of required and optional checks as well as merge criteria that must be met before a pull request can merge.
When any of these checks fail, the GitHub Prow bot will leave a comment on the PR with links to the run of that check that failed.

As the PR author, it is your responsibility to evaluate the failed checks and determine if there are any changes necessary to pass the checks.
If you suspect that the check failure was a flake, you can trigger retests by commenting `/retest` (or `/retest-required` for retesting only the required checks) on the PR.

**Common Prow Commands**:
- `/retest` - Rerun all failed tests
- `/retest-required` - Rerun only required failed tests
- `/test <job-name>` - Run a specific test job

### Verifying your changes / Creating an OpenShift cluster from a PR

As part of merging a PR, there is a requirement to verify that the changes you've made are working as expected using the `/verified` comment command.

While there are a lot of scenarios where the existing CI/CD checks may be sufficient to verify your changes are working (and can be denoted by commenting `/verified by ci`),
there may be scenarios where manual verification is required.

You can use the `Cluster Bot` Slack App to create a cluster from a PR by sending it a message in the format of `launch ${OCP_VERSION},${PR_LINK} ${PLATFORM},${VARIANT}`.
As an example, `launch 4.22,https://github.com/openshift/cluster-policy-controller/pull/123 aws` would launch an OpenShift 4.22 cluster with the changes made in the PR running on AWS.
For more information on what `Cluster Bot` can do, you can send it a message saying `help` and it will respond with additional documentation on how it can be used.

Once you've verified your changes work as expected, you can mark the PR as verified by commenting `/verified by @{your_github_handle}` on the PR.

### Additional Resources

For more information regarding more general OpenShift pull request processes, the following resources are helpful:

- https://docs.ci.openshift.org/architecture/jira
- https://docs.ci.openshift.org/
- https://steps.ci.openshift.org/

---

## Review Expectations

### Requesting a review

If you are not a member of the OpenShift control plane team and you need a review on a PR, post it in the [#forum-ocp-apiserver](https://redhat.enterprise.slack.com/archives/CB48XQ4KZ) Slack channel or reach out to folks outlined in the OWNERS file directly.

If you are a member of the OpenShift control plane team, reviews should come from your feature team. In the event your feature team does not have someone that can approve a PR, post it in the [#control-plane](https://redhat.enterprise.slack.com/archives/CC3CZCQHM) Slack channel.

OpenShift uses AI code review tools as part of the code review process.
Before requesting a review, address all feedback from the code review agent(s).
It is up to your discretion as the contributor how you would like to address that feedback.
Responding with an explanation as to why you are not going to take action on a comment made by the agent is an acceptable way to "address" its feedback.

### Interacting with reviewers

When interacting with reviewers/approvers:

- Be professional.
- Be respectful of differing opinions, viewpoints, and experiences.
- Gracefully give and receive constructive feedback.
- Focus on what is best for the product/organization, not just us as individuals.

A special note on the usage of AI - to respect the time of those that are reviewing your contribution, please do not use AI to respond to review comments.

---

## Local Development Guide

This section provides detailed guidance for local development, building, and testing the cluster-policy-controller.

### Prerequisites

#### Required Tools

- **Go**: Version specified in `go.mod` — [Installation guide](https://go.dev/doc/install)
- **Podman or Docker**: For building container images
- **oc CLI**: OpenShift command-line tool
- **Git**: For version control with commit signing enabled
- **Make**: For build automation

#### Verify Prerequisites

```bash
# Check Go version
go version  # Should match the version in go.mod

# Check cluster access (for deployment testing)
oc version
oc whoami

# Verify commit signing is enabled
git config --get user.signingkey
git config --get commit.gpgsign  # Should return 'true'
```

### Getting Started

#### Fork and Clone

1. **Fork the repository** on GitHub

2. **Clone your fork**:
   ```bash
   git clone https://github.com/<your-username>/cluster-policy-controller.git
   cd cluster-policy-controller
   ```

3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/openshift/cluster-policy-controller.git
   git fetch upstream
   ```

#### Understand the Codebase

Review the documentation:
- [AGENTS.md](./AGENTS.md) - Code patterns and conventions for AI agents
- [ARCHITECTURE.md](./ARCHITECTURE.md) - System design and architecture
- [README.md](./README.md) - Quick overview

Key directories:
```
cluster-policy-controller/
├── cmd/cluster-policy-controller/  # Main entrypoint
├── pkg/cmd/                        # Controller initialization
├── pkg/security/                   # UID/MCS allocation
├── pkg/quota/                      # Quota controllers
├── pkg/psalabelsyncer/             # PSA label sync
└── vendor/                         # Vendored dependencies
```

### Building

#### Build the Binary

```bash
# Build the binary
make build

# The binary is created at:
# ./cluster-policy-controller
```

#### Build the Container Image

```bash
# Build using the RHEL Dockerfile
podman build -f Dockerfile.rhel -t cluster-policy-controller:dev .
```

### Testing Your Changes

#### Unit Tests

```bash
# Run all unit tests
make test

# Run specific package tests
go test ./pkg/security/...
go test ./pkg/psalabelsyncer/...
go test ./pkg/quota/...

# Run with verbose output
go test -v ./pkg/security/controller/...

# Run with coverage
go test -coverprofile=coverage.out ./pkg/... ./cmd/...
go tool cover -html=coverage.out
```

#### Verification

```bash
# Run all verification checks
make verify
```

### Deploying Changes to a Cluster

The cluster-policy-controller runs inside the kube-controller-manager static pod. To test changes:

1. **Build and push a container image**:
   ```bash
   export QUAY_USER=<your-quay-username>
   export IMAGE_TAG=dev-$(git rev-parse --short HEAD)

   podman build -f Dockerfile.rhel -t quay.io/${QUAY_USER}/cluster-policy-controller:${IMAGE_TAG} .
   podman push quay.io/${QUAY_USER}/cluster-policy-controller:${IMAGE_TAG}
   ```

2. **Follow the OpenShift test deployment guide**:
   See [How can I test changes to an OpenShift operator/operand/release component?](https://github.com/openshift/enhancements/blob/master/dev-guide/operators.md#how-can-i-test-changes-to-an-openshift-operatoroperandrelease-component)

3. **Or use Cluster Bot**:
   Send the PR link to the Cluster Bot Slack App to launch a test cluster with your changes.

### Debugging

#### View Logs

The binary runs as a container in the kube-controller-manager static pod:

```bash
# Find the pod
oc get pods -n openshift-kube-controller-manager

# View logs for the cluster-policy-controller container
oc logs -n openshift-kube-controller-manager \
  <kube-controller-manager-pod> \
  -c cluster-policy-controller -f
```

#### Check Controller Status

```bash
# Check namespace UID allocation
oc get namespace <name> -o jsonpath='{.metadata.annotations}'

# Check ClusterResourceQuota status
oc get clusterresourcequota -o yaml

# Check PSA labels on a namespace
oc get namespace <name> --show-labels

# Check CSR approval
oc get csr
```

#### Common Issues

**Issue**: Namespace missing UID range annotation
```bash
# Check if the SCC allocation controller is running
oc logs -n openshift-kube-controller-manager \
  <pod> -c cluster-policy-controller | grep "namespace-security-allocation"

# Check RangeAllocation
oc get rangeallocations scc-uid -o yaml
```

**Issue**: PSA labels not being set
```bash
# Check if the namespace has the opt-in label
oc get namespace <name> -o jsonpath='{.metadata.labels.security\.openshift\.io/scc\.podSecurityLabelSync}'

# Check if the namespace has a UID range annotation (required)
oc get namespace <name> -o jsonpath='{.metadata.annotations.openshift\.io/sa\.scc\.uid-range}'
```

---

## Additional Resources

- [OpenShift Operator Best Practices](https://github.com/openshift/enhancements/blob/master/dev-guide/operators.md)
- [Kubernetes Controller Best Practices](https://kubernetes.io/docs/concepts/architecture/controller/)
- [OpenShift CI Documentation](https://docs.ci.openshift.org/)
- [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/)
- [OpenShift Security Context Constraints](https://docs.openshift.com/container-platform/latest/authentication/managing-security-context-constraints.html)

---

**Questions?**

- Post in [#forum-ocp-apiserver](https://redhat.enterprise.slack.com/archives/CB48XQ4KZ) Slack channel
- Open an issue on GitHub
- Review the OWNERS file for maintainer contacts

**Thank you for contributing!**
