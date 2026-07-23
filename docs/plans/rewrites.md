# Rewrites

`alog` is designed for forward-only appends. There are valid use cases
for modifying existing data, notably for payload repair, ordering and completeness
corrections.

This document aims to describe the parameters of an eventual design.

## Desired Characteristics

- Predictable normal path: common append behavior and performance stay unchanged.
- Strong safety: crash/restart should recover to one unambiguous valid state.
- Sequence consistency: resulting sequence space remains contiguous and valid.
- Minimal rewrite scope: avoid multi-segment payload rewrites when possible.
- Bounded cost: rewrite effort should be measurable and configurable.
- Strict disk guarantees: `MaxDiskSize` must ideally hold during the full operation, 
  not just after completion.
- Operational clarity: failure modes should be explicit and easy to reason about.

## Design Tensions

- Safety vs space: copy-on-write is safer but may require temporary overlap.
- Space vs complexity: in-place rewrite can reduce bytes, but recovery semantics
  get harder.
- Throughput vs determinism: rewrite should be exclusive, but lock duration must
  stay operationally reasonable.
- Sequence shifts vs external references: shifted sequences may invalidate
  downstream assumptions.

## Expected Challenges

- Preserving `MaxDiskSize` at every point in a rewrite transaction.
- Managing metadata updates for later sequence shifts without broad data copying.
- Designing reader behavior during active rewrite (pause/retry/invalidate).
- Handling partial progress and interruption without ambiguous on-disk state.
- Keeping implementation complexity proportional to expected feature usage.

