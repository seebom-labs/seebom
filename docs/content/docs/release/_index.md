---
title: "Release"
linkTitle: "Release"
type: docs
weight: 5
description: >
  Release process, CI workflows, container images, and Helm chart publishing.
---
## Container Images
All images are published to **GitHub Container Registry (ghcr.io)**.
Images are built for **linux/amd64** and **linux/arm64**.
## How to Release
```bash
git tag v0.1.2
git push origin v0.1.2
```
The GitHub Actions workflow triggers on any `v*` tag and builds all 5 images (multi-arch), pushes the Helm chart as an OCI artifact, and creates a GitHub Release.
## Installing from a Release
```bash
helm install seebom oci://ghcr.io/seebom-labs/seebom/charts/seebom \
  --version 0.1.2 \
  -f values-production.yaml
```
## CI Workflows
| Workflow | Trigger | What it does |
|----------|---------|-------------|
| CI | Push/PR to main | Go build + test + vet, Angular build, Helm lint |
| Release | Git tag v* | Build + push images, Helm chart, GitHub Release |
| Pre-Release | Manual | Build from any branch, create pre-release |
## Building Images Locally
```bash
make images            # Build all 5 images with tag "dev"
make images TAG=0.2.0  # Build with a specific tag
make images-push       # Build and push to GHCR
```
## Versioning
- **Git tags**: v0.1.2 (SemVer)
- **Image tags**: 0.1.3 (without v) + latest
- **Helm chart version**: Matches the Git tag
