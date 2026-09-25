//go:build e2e

package e2e

// The definitions here are the ones the suite triggers. They are deliberately the
// same shape as infrastructure/dev/jobdefs: a queue, a runtime, a command and a typed
// parameter schema. Job bodies run in bash:5 because it is small and has a shell the
// runner can invoke; the alpine definition exists to prove the runner works with the
// images the README actually advertises.

// echoBuildYAML prints its resolved parameters, so a test can read them back out of
// the build log and prove they reached the container as environment variables.
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

// failingBuildYAML exits non-zero, which must surface as a failed build.
const failingBuildYAML = `
name: Failing Build
description: Exits non-zero.
queue: build
runtime: bash:5
command: |
  echo "about to fail"
  exit 3
`

// slowBuildYAML runs long enough for a test to catch it mid-flight, kill its agent,
// or watch a second build queue behind it.
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

// regionBuildYAML only runs on an agent labelled for the region, so a test can prove
// core withholds work from hosts that do not match.
const regionBuildYAML = `
name: Region Build
description: Requires an agent labelled region=eu.
queue: build
labels:
  region: eu
runtime: bash:5
command: echo "ran in eu"
`

// deployQueueYAML sits on another queue, to prove an agent only pulls what it asked for.
const deployQueueYAML = `
name: Deploy Only
description: Lives on the deploy queue.
queue: deploy
runtime: bash:5
command: echo "deployed"
`

// alpineBuildYAML uses the kind of image the README advertises. Alpine ships
// /bin/sh and no bash, so this definition is the one that says whether Lute can
// really run the runtimes it documents.
const alpineBuildYAML = `
name: Alpine Build
description: Runs on an alpine image, as the documented examples do.
queue: build
runtime: alpine:3
command: echo "alpine ran"
`

// Slugs of the definitions above, as slugify derives them from the names.
const (
	echoSlug    = "echo-build"
	failingSlug = "failing-build"
	slowSlug    = "slow-build"
	regionSlug  = "region-build"
	deploySlug  = "deploy-only"
	alpineSlug  = "alpine-build"
)

// allJobDefs is every definition file a stack starts with, keyed by file name.
var allJobDefs = map[string]string{
	"echo-build.yaml":    echoBuildYAML,
	"failing-build.yaml": failingBuildYAML,
	"slow-build.yaml":    slowBuildYAML,
	"region-build.yaml":  regionBuildYAML,
	"deploy-only.yaml":   deployQueueYAML,
	"alpine-build.yaml":  alpineBuildYAML,
}
