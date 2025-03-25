# Performance Metrics

## Summary

| Metric                    | Value               |
|---------------------------|---------------------|
| **Total Requests**        | 21,674              |
| **Avg Response Time**     | 2 ms                |
| **P99 Response Time**     | 14 ms (GET), 14 ms (PUT) |
| **Max Response Time**     | 1,017 ms (GET), 57 ms (PUT) |
| **Error Rate**            | 0.00% (No failures) |
| **Request Throughput**    | 236.39 req/s        |

---

### Request Statistics

| Type  | Name                 | # Requests | # Fails (%) | Avg (ms) | Min (ms) | Max (ms) | Med (ms) | Req/s  | Failures/s |
|-------|----------------------|------------|-------------|----------|----------|----------|----------|--------|------------|
| GET   | `/get?key=test`      | 15,163     | 0 (0.00%)   | 2        | 0        | 1,017    | 2        | 165.37 | 0.00       |
| POST  | `/put`               | 6,511      | 0 (0.00%)   | 2        | 0        | 57       | 2        | 71.01  | 0.00       |
|       | **Aggregated**       | 21,674     | 0 (0.00%)   | 2        | 0        | 1,017    | 2        | 236.39 | 0.00       |

---

### Response Time Percentiles (Approximated)

| Type  | Name                 | 50%  | 66%  | 75%  | 80%  | 90%  | 95%  | 98%  | 99%  | 99.9% | 99.99% | 100%  | # Requests |
|-------|----------------------|------|------|------|------|------|------|------|------|-------|--------|-------|------------|
| GET   | `/get?key=test`      | 2    | 2    | 3    | 3    | 5    | 8    | 11   | 14   | 32    | 1000   | 1000  | 15,163     |
| POST  | `/put`               | 2    | 2    | 3    | 4    | 5    | 8    | 12   | 14   | 30    | 57     | 57    | 6,511      |
|       | **Aggregated**       | 2    | 2    | 3    | 3    | 5    | 8    | 12   | 14   | 31    | 1000   | 1000  | 21,674     |
