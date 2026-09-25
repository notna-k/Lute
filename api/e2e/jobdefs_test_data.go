//go:build e2e

package e2e

// Definitions the suite triggers, shaped like infrastructure/dev/jobdefs. Bodies run in
// bash:5; the alpine one proves the images the README advertises work.

// echoBuildYAML prints its parameters, proving they reached the container as env vars.
const echoBuildYAML = `
name: Echo Build
description: Prints its parameters so a test can read them back.
queue: build
runtime: bash:5
command: |
  echo "environment=${ENVIRONMENT}"
  echo "regions=${REGIONS}"
  echo "dry_run=${DRY_RUN}"
  echo "retries=${RETRIES}"
  echo "build finished"
parameters:
  - name: environment
    type: select
    label: Target environment
    env: ENVIRONMENT
    required: true
    default: staging
    options:
      - { value: staging, label: Staging }
      - { value: prod, label: Production }
  - name: regions
    type: multiselect
    label: Regions
    env: REGIONS
    default: [eu-central]
    options:
      - { value: eu-central, label: eu-central }
      - { value: us-east, label: us-east }
  - name: dry_run
    type: bool
    label: Dry run
    env: DRY_RUN
    default: true
  - name: retries
    type: number
    label: Retries
    env: RETRIES
    default: 2
  - name: deploy_token
    type: secret
    label: Deploy token
    env: DEPLOY_TOKEN
    secretRef: secrets/deploy
`

const failingBuildYAML = `
name: Failing Build
description: Exits non-zero.
queue: build
runtime: bash:5
command: |
  echo "about to fail"
  exit 3
`

// slowBuildYAML runs long enough to be caught mid-flight.
const slowBuildYAML = `
name: Slow Build
description: Sleeps, so a test can observe a build in progress.
queue: build
runtime: bash:5
command: |
  echo "slow build started"
  sleep 30
  echo "slow build finished"
`

// regionBuildYAML needs an agent labelled for its region.
const regionBuildYAML = `
name: Region Build
description: Requires an agent labelled region=eu.
queue: build
labels:
  region: eu
runtime: bash:5
command: echo "ran in eu"
`

const deployQueueYAML = `
name: Deploy Only
description: Lives on the deploy queue.
queue: deploy
runtime: bash:5
command: echo "deployed"
`

// alpineBuildYAML runs on an image with /bin/sh and no bash, as the README advertises.
const alpineBuildYAML = `
name: Alpine Build
description: Runs on an alpine image, as the documented examples do.
queue: build
runtime: alpine:3
command: echo "alpine ran"
`

const (
	echoSlug    = "echo-build"
	failingSlug = "failing-build"
	slowSlug    = "slow-build"
	regionSlug  = "region-build"
	deploySlug  = "deploy-only"
	alpineSlug  = "alpine-build"
)

var allJobDefs = map[string]string{
	"echo-build.yaml":    echoBuildYAML,
	"failing-build.yaml": failingBuildYAML,
	"slow-build.yaml":    slowBuildYAML,
	"region-build.yaml":  regionBuildYAML,
	"deploy-only.yaml":   deployQueueYAML,
	"alpine-build.yaml":  alpineBuildYAML,
}
