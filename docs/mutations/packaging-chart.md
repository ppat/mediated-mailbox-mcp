# Mutations: packaging/chart

The demonstrations of the controls whose patches sit in `packaging/chart/`. [MUTATIONS.md](../MUTATIONS.md) defines a row, its lifecycle and which file holds it.

## A configuration change restarts only the processes that read it

- **Date · evidence:** 2026-10-12 · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes and the soft memory limits changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** the API server's and the UI's pods carry no checksum, so a change to their files restarts nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestAConfigurationChangeRestartsOnlyTheProcessesThatReadIt`
- **Break (2):** the worker's checksum hashes every process's file, so any change restarts it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestAConfigurationChangeRestartsOnlyTheProcessesThatReadIt`

## Each Go container gets only its own soft memory limit

- **Date · evidence:** 2026-10-13 · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), at RAPID_SEED=1, and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** the migration step's container never gets the soft memory limit set for it
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestEachGoContainerGetsOnlyItsOwnSoftMemoryLimit`
- **Break (2):** the UI's container takes the API server's soft memory limit rather than its own
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestEachGoContainerGetsOnlyItsOwnSoftMemoryLimit`

## Every pod reads only the secrets its process uses

- **Date · evidence:** 2026-10-12 · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes and the soft memory limits changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** the API server's pod also mounts the migration role's password file
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestEveryPodReadsOnlyTheSecretsItsProcessUses`
- **Break (2):** the worker mounts each role's whole Secret rather than the one key it reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestEveryPodReadsOnlyTheSecretsItsProcessUses`

## Every pod the chart renders runs hardened

- **Date · evidence:** 2026-10-12 · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes and the soft memory limits changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and break 1 again on 2026-10-14 at RAPID_SEED=1, after the Helm test's checks of the UI moved under `/ui`, which changed the file its patch edits · [pull request #383](https://github.com/ppat/mediated-mailbox-mcp/pull/383), and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** the Helm test's pod runs with no pod or container security context
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestEveryPodRunsHardened`
- **Break (2):** every container's root filesystem is writable and the API server mounts its ServiceAccount token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestEveryPodRunsHardened`

## Nothing optional renders by default

- **Date · evidence:** 2026-10-12 · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes and the soft memory limits changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** an enabled Ingress renders nothing
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestNothingOptionalRendersByDefault`
- **Break (2):** the API server's Ingress is enabled by default
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestNothingOptionalRendersByDefault`

## The bearer token and the request token key come from the user's Secrets, or from one the chart creates and keeps

- **Date · evidence:** 2026-10-12 · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes and the soft memory limits changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** the access credentials Secret carries no keep policy, so uninstalling the release deletes the bearer token
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheBearerTokenAndRequestTokenKeyComeFromTheUsersSecretsOrTheChartsOwn`
- **Break (2):** the chart renders its access credentials Secret for both values and the pods read it even when the user names a Secret for each
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheBearerTokenAndRequestTokenKeyComeFromTheUsersSecretsOrTheChartsOwn`
- **Break (3):** the chart renders its access credentials Secret only when no Secret is named for the request token key, so naming only that leaves the API server's bearer token with no Secret
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheBearerTokenAndRequestTokenKeyComeFromTheUsersSecretsOrTheChartsOwn`
- **Break (4):** the pods read the bearer token and the request token key under the default key names whatever key names the user gives
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheBearerTokenAndRequestTokenKeyComeFromTheUsersSecretsOrTheChartsOwn`

## The migration step is a hook that reads nothing the chart renders and carries no password

- **Date · evidence:** 2026-10-12 · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes and the soft memory limits changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** the migration Job runs after the install and the upgrade rather than before
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheMigrationStepIsAHookThatReadsNothingTheChartRenders`
- **Break (2):** the connection string names no password file, and the password comes from the Secret as PGPASSWORD
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestEveryPodReadsOnlyTheSecretsItsProcessUses`, `TestTheMigrationStepIsAHookThatReadsNothingTheChartRenders`
- **Break (3):** the migration Job mounts the chart's ConfigMap and replaces a pod while it still terminates
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheMigrationStepIsAHookThatReadsNothingTheChartRenders`

## The private key reaches only the processes that open credentials and the UI

- **Date · evidence:** 2026-10-12 · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes and the soft memory limits changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** the UI's pod mounts the public key and no private key
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestEveryPodReadsOnlyTheSecretsItsProcessUses`
- **Break (2):** the migration step's pod mounts the key pair too
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestEveryPodReadsOnlyTheSecretsItsProcessUses`

## The Prometheus Operator's resources render where its API is served or a value forces them

- **Date · evidence:** 2026-10-12 · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes and the soft memory limits changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** auto reads the group version rather than the kind, and true no longer forces a resource on
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestThePrometheusOperatorsResourcesFollowItsAPIAndTheSwitch`
- **Break (2):** the ServiceMonitors render whether or not the cluster serves their kind
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestNothingOptionalRendersByDefault`, `TestThePrometheusOperatorsResourcesFollowItsAPIAndTheSwitch`

## The values schema refuses a value Helm or a process would change

- **Date · evidence:** 2026-10-12 · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, with break 3 added, after the pool sizes and the soft memory limits changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** the API server's soft memory limit admits Kubernetes' units, which the Go runtime refuses at start
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheSchemaRefusesWhatHelmOrTheProcessesWouldChange`
- **Break (2):** the trigger map's key guard and its words' type are dropped, so an unquoted no or yes passes as false or true
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheSchemaRefusesWhatHelmOrTheProcessesWouldChange`
- **Break (3):** the API server's own block accepts any key
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheSchemaRefusesWhatHelmOrTheProcessesWouldChange`

## The worker runs as one pod, never two at once, a rollout included

- **Date · evidence:** 2026-10-12 · [pull request #357](https://github.com/ppat/mediated-mailbox-mcp/pull/357), and every break again on 2026-10-13 at RAPID_SEED=1, after the pool sizes and the soft memory limits changed the code, tests or patches the row rests on · [pull request #371](https://github.com/ppat/mediated-mailbox-mcp/pull/371), and every break again on 2026-10-13 at RAPID_SEED=1, after the rename of the API server and the migrator moved the code, tests or patches the row rests on · [pull request #376](https://github.com/ppat/mediated-mailbox-mcp/pull/376), in run [38440944897](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38440944897) of the `mutation-demonstrations` workflow, and every break again on 2026-10-14 at RAPID_SEED=1, after the host and the component seams changed the code, tests or patches the row rests on · [pull request #384](https://github.com/ppat/mediated-mailbox-mcp/pull/384)
- **Break (1):** the schema admits a replica count for the worker, which the template reads
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheWorkerRunsAsOnePodNeverTwo`
- **Break (2):** the worker's rollout starts the new pod before it stops the old one
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheWorkerRunsAsOnePodNeverTwo`
- **Break (3):** the worker's Deployment runs two replicas
  - **Went red in `github.com/ppat/mediated-mailbox-mcp/packaging/chart`:** `TestTheWorkerRunsAsOnePodNeverTwo`
