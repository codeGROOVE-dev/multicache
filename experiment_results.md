# CDN Hit Rate Optimization Experiments

## Baseline
- **Date**: 2024-12-30
- **Metric**: CDN hit rate average across 16K-256K cache sizes
- **Goal**: 58.30%
- **Current**: 57.90%

## Parameters Under Test
| Parameter | Current Value | Description |
|-----------|---------------|-------------|
| smallQueueRatio | 900 (90%) | Small queue size as per-mille of capacity |
| maxFreq | 2 | Frequency counter cap for eviction |
| ghostCapMultiplier | 8x | Ghost queue capacity multiplier |
| demotionThreshold | peakFreq >= 1 | Threshold for demotion from main to small |
| evictionThreshold | freq < 2 | Threshold for eviction from small queue |

---

## Experiment 1: Smaller Small Queue (80% instead of 90%)

**Hypothesis**: CDN traces have scan patterns. A smaller small queue protects the main queue better, keeping valuable items longer.

**Change**: `smallQueueRatio = 800` (from 900)

**Results**:
```
| Cache         |    16K |    32K |    64K |   128K |   256K |     Avg |
|---------------|--------|--------|--------|--------|--------|---------|
| multicache    | 55.46% | 57.09% | 58.47% | 59.59% | 60.55% | 58.23%  |

Delta: +0.33% (57.90% → 58.23%)
```

**Verdict**: ✓ IMPROVED - Closer to goal but not quite there

---

## Experiment 2: Higher maxFreq (3 instead of 2)

**Hypothesis**: Requiring more accesses before incrementing freq counter might help filter out one-hit-wonders.

**Change**: `maxFreq = 3` (from 2)

**Results**:
```
CDN Avg: 57.90%
Delta: 0.00% (no change)
```

**Verdict**: ✗ NO EFFECT

**Note**: Also discovered that setting `maxFreq = 1` creates an infinite loop in eviction (items with freq=1 get promoted instead of evicted, causing evictFromSmall to never return true). Added warning comment.

---

## Experiment 3: Larger Ghost Queue (12x instead of 8x)

**Hypothesis**: CDN has high churn (~768K unique keys for 2M ops). A larger ghost queue remembers more evicted keys, allowing better admission decisions.

**Change**: `ghostCap = size * 12` (from `size * 8`)

**Results**:
```
CDN Avg: 57.90%
Delta: 0.00% (no change)
```

**Verdict**: ✗ NO EFFECT

---

## Experiment 4: Higher Demotion Threshold (peakFreq >= 2 instead of >= 1)

**Hypothesis**: Only demoting items with higher historical frequency from main to small might keep the small queue cleaner.

**Change**: `if e.peakFreq.Load() >= 2` instead of `>= 1` in evictFromMain

**Results**:
```
CDN Avg: 57.79%
Delta: -0.11% (hurt performance)
```

**Verdict**: ✗ WORSE - Demotion helps CDN

---

## Experiment 5: Combined - 80% Small Queue + 6x Ghost

**Hypothesis**: Combining the winning 80% small queue with a smaller ghost might further improve CDN.

**Changes**:
- `smallQueueRatio = 800`
- `ghostCap = size * 6`

**Results**:
```
CDN Avg: 58.23%
Delta: +0.33% (same as Exp 1)
```

**Verdict**: ~ NEUTRAL - Ghost size change had no effect on top of 80% small queue

---

## Bonus Experiment: 75% Small Queue

**Hypothesis**: If 80% helped, maybe 75% helps more.

**Change**: `smallQueueRatio = 750`

**Results**:
```
CDN Avg: 58.34%
Delta: +0.44%
Goal: 58.30% ✓ ACHIEVED
```

**Verdict**: ✓ ACHIEVED CDN GOAL

**Caveat**: This hurts the overall hitrate average (58.34% < 59.00% goal) so cannot be adopted globally.

---

## Summary

| Experiment | CDN Avg | Delta | Meets Goal? |
|------------|---------|-------|-------------|
| Baseline | 57.90% | - | ✗ |
| Exp 1: Small Queue 80% | 58.23% | +0.33% | ✗ |
| Exp 2: maxFreq=3 | 57.90% | 0.00% | ✗ |
| Exp 3: Ghost 12x | 57.90% | 0.00% | ✗ |
| Exp 4: Demotion >= 2 | 57.79% | -0.11% | ✗ |
| Exp 5: 80% small + 6x ghost | 58.23% | +0.33% | ✗ |
| **Bonus: Small Queue 75%** | **58.34%** | **+0.44%** | **✓** |

## Key Findings

1. **Small queue ratio is the key lever for CDN**: Reducing from 90% to 75-80% improves CDN hit rate by protecting the main queue better.

2. **Ghost queue size doesn't matter for CDN**: Neither 6x nor 12x changed the result compared to 8x.

3. **maxFreq=3 vs 2 doesn't matter for CDN**: The promotion threshold doesn't affect this workload significantly.

4. **Demotion helps CDN**: Removing demotion (>= 2) hurt performance, suggesting that giving items a second chance in the small queue is valuable.

5. **Trade-off exists**: While 75% small queue meets CDN goal (58.34%), it fails the overall hitrate average goal (need 59.00%). The current 90% setting optimizes for the average across all workloads.

---

## Binary Search: Optimal smallQueueRatio for Overall Hitrate

**Goal**: Find smallQueueRatio that maximizes overall average hitrate across all 9 workloads.

**Method**: Binary search with SUITES=hitrate benchmark

| Ratio | Overall Avg | Notes |
|-------|-------------|-------|
| 950 | 58.84% | Worse |
| 900 | 59.40% | Baseline |
| 850 | 59.82% | Better |
| 800 | 60.03% | Better |
| 750 | 60.24% | Better |
| 700 | 60.38% | Better |
| 650 | 60.48% | Better |
| 600 | 60.55% | Better |
| 550 | 60.61% | Better |
| 500 | 60.64% | Better |
| 450 | 60.66% | Better |
| **400** | **60.68%** | **Optimal** |
| 375 | 60.68% | Plateau |
| 350 | 60.68% | Plateau |
| 325 | 60.68% | Plateau |
| 300 | 60.67% | Decline starts |
| 250 | 60.64% | Worse |

**Finding**: Optimal plateau at 325-400 (all achieve 60.68%). Selected 400 as the final value.

**Improvement**: +1.28% absolute (59.40% → 60.68%)

## Final Recommendations

1. **Changed smallQueueRatio from 900 to 400** (90% → 40% small queue)
   - Improves overall hitrate from 59.40% to 60.68% (+1.28%)
   - CDN: 58.63% (was 57.90%, +0.73%)
   - All workloads improved
