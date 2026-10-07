# 0117. The chart runs delta sync as a StatefulSet of one replica, so a second pod starts only once the first is gone

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox), [O1](../../../USE_CASES.md#o1--rate-limited-politely)

## Context

Delta sync is one process that runs until stopped, and it assumes the platform runs one at a time,
a rollout included ([ADR-0103](./0103-delta-sync-runs-continuously-and-ticks-on-the-sync-interval.md)).
Its ticks do not overlap within a process, and a tick that starts records as failed the account's
latest tick still recorded as running, so a second process would fail the first one's ticks and
spend the account's rate budget twice for the same changes. The chart of
[ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md) chooses the Kubernetes object
that keeps that assumption, and Kubernetes offers two that run one replica.

A Deployment with the `Recreate` strategy deletes its old pod and waits for it to end before it
starts the new one during a rollout. Outside a rollout, when a node stops answering, Kubernetes
marks the node's pods for deletion after a timeout, and the Deployment's ReplicaSet starts a
replacement while the old pod may still be running on the node it can no longer reach. A
StatefulSet starts a pod of a given identity only once the pod before it is confirmed gone, in a
rollout and when a node stops answering alike. Kubernetes' documentation states its cost: a rollout
to a pod template whose pod never becomes ready, such as one whose configuration refuses the start,
stops, and after the template is put right, the stuck pod must be deleted by hand before the rollout
continues.

## Decision

- **Delta sync runs as a StatefulSet of one replica**, with ordered pod management and the rolling
  update strategy, so a second delta sync pod starts only once the first is gone.
- **No other object the chart renders runs delta sync's image.**
- **Its Service exposes only the probes' port**, and is the StatefulSet's governing Service.

## Alternatives considered

- **A Deployment of one replica with the `Recreate` strategy.** For it, the usual object for a
  stateless process, and a rollout to a corrected template replaces a pod that never became ready
  with no step by hand. Against it, a node that stops answering lets a second process start beside
  the first, which ADR-0103 rules out. Not chosen, so the rule holds on any cluster the chart lands
  on. A deployment that runs on one node, where that case cannot arise, loses nothing by the
  StatefulSet except the cost above.
- **A Deployment with the rolling update strategy.** No case was tabled for it. Rejected, because a
  rollout starts the new pod before the old one ends.
- **A lock in the database that a second process waits on.** For it, the rule would hold whatever
  runs the process. Against it, the application would carry a coordination mechanism for a case the
  platform's object already rules out, and ADR-0103 places the rule with the platform. Not chosen.

## Consequences

- A rollout to a template whose delta sync pod never becomes ready stops. Once the template is put
  right, the operator deletes the stuck pod, and the rollout continues. The configuration library
  refuses a mistaken value at start ([ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md)),
  which is the likeliest way to reach this case.
- A node that stops answering leaves delta sync stopped until Kubernetes confirms the old pod gone,
  which can take until the node returns or someone deletes the pod by force. A forced delete starts
  the replacement at once whether or not the old process still runs, so on a node that is only
  partitioned it runs two delta sync processes, which this record exists to prevent. It is safe only
  once the node, or the old pod's process, is known to have stopped. The ticks it misses are
  caught up by the next one, and a gap beyond the provider's change history is recovered and raised
  ([ADR-0018](../data/0018-delta-sync-polls.md),
  [ADR-0105](../data/0105-a-cursor-gap-is-recovered-from-the-last-cursors-write-time.md)).
- Assumptions about other components. Kubernetes keeps a StatefulSet's guarantee of at most one pod
  per identity. Nothing else runs delta sync's image against the same database.
- The rule above is a control. Its injection is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
