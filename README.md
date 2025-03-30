# Performance Metrics

## Summary

| Metric                    | Value               |
|---------------------------|---------------------|
| **Total Requests**        | 55,475              |
| **Avg Response Time**     | 3 ms                |
| **P99 Response Time**     | 77 ms (GET), 59 ms (POST) |
| **Max Response Time**     | 109,925 ms (GET), 109,901 ms (POST) |
| **Error Rate**            | 13.51% (Failures)   |
| **Request Throughput**    | 560.07 req/s        |

---

### Request Statistics

| Type  | Name                 | # Requests | # Fails (%) | Avg (ms) | Min (ms) | Max (ms) | Med (ms) | Req/s  | Failures/s |
|-------|----------------------|------------|-------------|----------|----------|----------|----------|--------|------------|
| GET   | `/get?key=test`      | 38,846     | 0 (0.00%)   | 3        | 0        | 77       | 2        | 323.97 | 0.00       |
| POST  | `/put`               | 16,629     | 0 (0.00%)   | 3        | 0        | 59       | 2        | 138.68 | 0.00       |
|       | **Aggregated**       | 55,475     | 0 (0.00%)   | 3        | 0        | 77       | 2        | 462.65 | 0.00       |

---

### Response Time Percentiles (Approximated)

| Type  | Name                 | 50%  | 66%  | 75%  | 80%  | 90%  | 95%  | 98%  | 99%  | 99.9% | 99.99% | 100%  | # Requests |
|-------|----------------------|------|------|------|------|------|------|------|------|-------|--------|-------|------------|
| GET   | `/get?key=test`      | 2    | 2    | 3    | 3    | 5    | 7    | 26   | 34   | 47    | 76     | 77    | 38,846     |
| POST  | `/put`               | 2    | 2    | 3    | 3    | 5    | 7    | 24   | 32   | 48    | 59     | 59    | 16,629     |
|       | **Aggregated**       | 2    | 2    | 3    | 3    | 5    | 7    | 26   | 34   | 48    | 70     | 77    | 55,475     |

---