# Scalability test

Is a test meant to detect regressions in the Kueue's overall scheduling capabilities. 

# Components
In order to achieve this the following components are used:

## Runner

An application able to:
- generate a set of Kueue specific objects based on a config following the schema of [generator.yaml](`./configs/baseline/generator.yaml`)
- mimic the execution of the workloads
- monitor the created object and generate execution statistics based on the received events

Optionally it's able to run an instance of [minimalkueue](#MinimalKueue) in a dedicated [envtest](https://book.kubebuilder.io/reference/envtest.html) environment.

## MinimalKueue

A light version of the Kueue's controller manager consisting only of the core controllers and the scheduler.

It is designed to offer the Kueue scheduling capabilities without any additional components which may flood the optional cpu profiles taken during it's execution.


## Checker

Checks the results of a performance-scheduler against a set of expected value defined as [rangespec.yaml](configs/baseline/rangespec.yaml).

# Usage

## Run in an existing cluster

```bash
make run-performance-scheduler-in-cluster
```

Will run a performance-scheduler scenario against an existing cluster (connectable by the host's default kubeconfig), and store the resulting artifacts are stored in `$(PROJECT_DIR)/bin/run-performance-scheduler-in-cluster`.

The generation config to be used can be set in `SCALABILITY_GENERATOR_CONFIG` by default using `$(PROJECT_DIR)/test/performance/scheduler/configs/baseline/generator.yaml`

Setting `SCALABILITY_SCRAPE_INTERVAL` to an interval value and `SCALABILITY_SCRAPE_URL` to an URL exposing kueue's metrics will cause the scalability runner to scrape that URL every interval and store the results in `$(PROJECT_DIR)/bin/run-performance-scheduler-in-cluster/metricsDump.tgz`.

Check [installation guide](https://kueue.sigs.k8s.io/docs/installation) for cluster and [observability](https://kueue.sigs.k8s.io/docs/installation/#add-metrics-scraping-for-prometheus-operator).

## Run with minimalkueue

```bash
make run-performance-scheduler
```

Will run a performance-scheduler scenario against an [envtest](https://book.kubebuilder.io/reference/envtest.html) environment
and an instance of minimalkueue.
The resulting artifacts are stored in `$(PROJECT_DIR)/bin/run-performance-scheduler`.

The generation config to be used can be set in `SCALABILITY_GENERATOR_CONFIG` by default using `$(PROJECT_DIR)/test/performance/scheduler/configs/baseline/generator.yaml`

Setting `SCALABILITY_CPU_PROFILE=1` will generate a cpuprofile of minimalkueue in `$(PROJECT_DIR)/bin/run-performance-scheduler/minimalkueue.cpu.prof`

Setting `SCALABILITY_KUEUE_LOGS=1` will save the logs of minimalkueue in  `$(PROJECT_DIR)/bin/run-performance-scheduler/minimalkueue.out.log` and  `$(PROJECT_DIR)/bin/run-performance-scheduler/minimalkueue.err.log`

Setting `SCALABILITY_SCRAPE_INTERVAL` to an interval value (e.g. `1s`) will expose the metrics of `minimalkueue` and have them collected by the scalability runner in `$(PROJECT_DIR)/bin/run-performance-scheduler/metricsDump.tgz` every interval. 

## Run performance-scheduler test

```bash
make test-performance-scheduler
```

Runs the performance-scheduler with minimalkueue and checks the results against `$(PROJECT_DIR)/test/performance/scheduler/configs/baseline/rangespec.yaml`

## Scrape result

The scrape result `metricsDump.tgz` contains a set of `<ts>.prometheus` files, where `ts` is the millisecond representation of the epoch time at the moment each scrape was stared and can be used during the import in a visualization tool.

If an instance of [VictoriaMetrics](https://docs.victoriametrics.com/) listening at `http://localhost:8428` is used, a metrics dump can be imported like:

```bash
 TMPDIR=$(mktemp -d)
 tar -xf ./bin/run-performance-scheduler/metricsDump.tgz -C $TMPDIR
 for file in ${TMPDIR}/*.prometheus; do timestamp=$(basename "$file" .prometheus);  curl -vX POST -T "$file" http://localhost:8428/api/v1/import/prometheus?timestamp="$timestamp"; done
 rm -r $TMPDIR

```

## TAS (Topology Aware Scheduling) Tests

The performance test framework supports TAS features using the same infrastructure with the `--enableTAS` flag. TAS tests measure performance with:
- Hierarchical topology (blocks → racks → nodes)
- Nodes with topology labels
- Workloads with topology constraints (required, preferred, balanced)

### Run TAS tests

```bash
# Run with minimalkueue in envtest
make run-tas-performance-scheduler

# Run in existing cluster
make run-tas-performance-scheduler-in-cluster

# Run and validate against thresholds
make test-tas-performance-scheduler
```

Same environment variables as standard tests apply (`SCALABILITY_CPU_PROFILE`, `SCALABILITY_KUEUE_LOGS`, etc.).
Results are stored in `$(PROJECT_DIR)/bin/run-tas-performance-scheduler/`.

### TAS configuration

TAS config (`configs/tas/generator.yaml`) extends the standard format with:

```yaml
topology:
  name: "default-topology"
  levels:
    - name: rack
      count: 10
      nodeLabel: "cloud.provider.com/topology-rack"
    - name: node
      count: 64
      nodeLabel: "kubernetes.io/hostname"
      capacity:
        cpu: "96"
        memory: "256Gi"

resourceFlavor:
  name: "tas-flavor"
  nodeLabel: "tas-node-group"

cohorts:
  # TAS workloads add:
  # - podCount: number of pods (default 1)
  # - tasConstraint: "required", "preferred", or "balanced"
  # - tasLevel: topology level label
  # - sliceSize: pods per slice for balanced placement
```

Performance thresholds are defined in `configs/tas/rangespec.yaml`.

## Unschedulable Workloads Test

Tests that high-priority inadmissible workloads do not block lower-priority
admissible ones from being scheduled. This validates the inadmissible workload
requeue frequency fix: previously, inadmissible workloads would repeatedly
enter the scheduling cycle and block admissible ones behind them, wasting
cluster capacity. After the fix, the scheduler requeues them less frequently,
letting admissible workloads through.

### Scenario

One cohort with two ClusterQueues:

- **borrower-cq**: No CPU capacity of its own, relies on borrowing. Has 1000
  pod capacity. Submits two kinds of workloads:
  - *needs-cpu* (priority 200): Requests 200m CPU, must borrow from the
    cohort. Because `capacity-cq` is fully utilizing its CPU, these remain
    permanently inadmissible. 200 of these are created to apply pressure.
  - *pods-only* (priority 50): No CPU request, should be admitted promptly
    against the pod quota **despite being lower priority** than the
    inadmissible workloads ahead of them in the queue.
- **capacity-cq**: Owns 100 CPU. Submits 500 workloads each requesting 200m
  CPU (total = 100 CPU), fully saturating its own quota. Nothing is left for
  `borrower-cq` to borrow.

Fair sharing is enabled so the scheduler evaluates fair share ratios when
making preemption decisions. Preemption is set to `reclaimWithinCohort: Any`
and `withinClusterQueue: LowerPriority` on both CQs.

### What the test validates

- **pods-only time-to-admission**: The key metric. If inadmissible workloads
  are blocking them, this will be very high or they'll never be admitted.
- **capacity-cq usage**: Should be near 100% — `cpu-hog` workloads fill it.
- **needs-cpu**: Intentionally excluded from admission time checks. They will
  never be admitted.

### Run unschedulable workload tests

```bash
# Run with minimalkueue in envtest
make run-unschedulable-performance-scheduler

# Run in existing cluster
make run-unschedulable-performance-scheduler-in-cluster

# Run and validate against thresholds
make test-unschedulable-performance-scheduler
```

Same environment variables as standard tests apply (`SCALABILITY_CPU_PROFILE`,
`SCALABILITY_KUEUE_LOGS`, etc.).
Results are stored in `$(PROJECT_DIR)/bin/run-unschedulable-performance-scheduler/`.

### Configuration

The config (`configs/unschedulable/generator.yaml`) uses the `resources` field
for per-resource quotas instead of the legacy single-resource `nominalQuota`
field:

```yaml
queuesSets:
  - className: borrower-cq
    resources:
      - name: pods
        nominalQuota: 1000
      - name: cpu
        nominalQuota: 0
        borrowingLimit: 100
    workloadsSets:
      # High priority but inadmissible — should not block the queue
      - count: 200
        workloads:
          - className: needs-cpu
            priority: 200
            request: 200m
      # Lower priority but admissible — should get through
      - count: 100
        workloads:
          - className: pods-only
            priority: 50
```

The `resources` list maps directly to Kueue's `FlavorQuotas.Resources`. Each
entry supports `name`, `nominalQuota`, `borrowingLimit`, and `lendingLimit`.
When `resources` is omitted, the legacy behavior applies (single CPU resource
from `nominalQuota`/`borrowingLimit`).

Workloads with no `request` field get no CPU resource request in their spec.
The scheduler automatically accounts for `pods` usage based on podSet count
when the CQ has `pods` in its resource group.

Performance thresholds are defined in `configs/unschedulable/rangespec.yaml`.
