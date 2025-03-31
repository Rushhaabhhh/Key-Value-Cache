# Key-Value Cache

A lightweight, high-performance, in-memory key-value store service with a clean HTTP API interface, implemented in Rust.

## Installation

### Prerequisites
- Rust 1.70 or higher
- Cargo (Rust's package manager)

### From Source
```bash
# Clone the repository
git clone https://github.com/Rushhaabhhh/Key-Value-Cache.git
cd Key-Value-Cache

# Build the binary
cargo build --release

# Run the server
./target/release/key_value_cache
```

### Using Cargo
```bash
# Install directly using cargo
cargo install --git https://github.com/Rushhaabhhh/Key-Value-Cache.git

# Run the server
key_value_cache
```

### Using Docker
```bash
# Build the Docker image
docker build -t key-value-cache .

# Run the container
docker run -p 7171:7171 key-value-cache
```

## Configuration

The application uses the following default values, which can be modified by creating a `.env` file in the project root:

```
PORT=7171
MAX_KEY_SIZE=256
MAX_VALUE_SIZE=256
```

## API Usage

### Store a Value
```
POST /put
```

Request body:
```json
{
  "key": "user:1234",
  "value": "John Doe"
}
```

Response:
```json
{
  "status": "OK",
  "message": "Key inserted/updated successfully."
}
```

### Retrieve a Value
```
GET /get?key=user:1234
```

Success Response:
```json
{
  "status": "OK",
  "key": "user:1234",
  "value": "John Doe",
  "message": "Key retrieved successfully."
}
```

Key Not Found Response:
```json
{
  "status": "ERROR",
  "message": "Key not found."
}
```

## Design Choices and Optimizations

### Memory Efficiency
- **LRU Cache Implementation**: Uses Rust's `LinkedHashMap` to efficiently track and remove the least recently used items when needed.
- **Automatic Memory Management**: Continuously monitors system memory usage and proactively evicts items when reaching 70% of available memory.
- **Progressive Eviction**: Implements a gradual eviction strategy that scales up batch sizes as needed, ensuring system responsiveness during cleanup operations.

### Performance Enhancements
- **Asynchronous Processing**: Built on Tokio's asynchronous runtime for efficient handling of concurrent requests without blocking.
- **Thread-Safe Design**: Uses Rust's thread safety guarantees with `Arc<RwLock<>>` for safe concurrent access to the cache.
- **Request Size Limits**: Enforces maximum key and value sizes to prevent abuse and ensure consistent performance.

### Reliability Features
- **Graceful Shutdown**: Implements proper shutdown handling to ensure no data is lost when the service is stopped.
- **Error Handling**: Leverages Rust's robust error handling to provide clear and consistent error responses.
- **Resource Monitoring**: Continuously adapts to changing workloads by monitoring and managing resource usage.

### Scalability
- **Horizontal Scaling**: Designed as a stateless service (except for the cache itself) for easy deployment behind a load balancer.
- **Low Memory Footprint**: Optimized for efficient memory usage, allowing deployment in resource-constrained environments.

## Performance

The service is highly performant:
- Handles 50,000+ requests per second on modest hardware
- P95 response times under 5ms
- Zero request failures under high concurrency (100+ simultaneous users)

### Load Test Results

```
█ THRESHOLDS 
  http_req_duration
  ✓ 'p(95)<500' p(95)=3.14ms

█ TOTAL RESULTS 
  checks_total.......................: 25468755 77177.190893/s
  checks_succeeded...................: 100.00%  25468755 out of 25468755
  checks_failed......................: 0.00%    0 out of 25468755
  
  CUSTOM
  cache_hits..............................................................: 8488190  25721.503072/s
  total_gets..............................................................: 8488190  25721.503072/s
  
  HTTP
  http_req_duration.......................................................: avg=1.09ms min=50µs    med=753µs    max=168.3ms  p(90)=2.37ms p(95)=3.14ms
  http_req_failed.........................................................: 0.00%    0 out of 16980565
  http_reqs...............................................................: 16980565 51455.687821/s
  
  EXECUTION
  iteration_duration......................................................: avg=1.3ms  min=79.04µs med=883.83µs max=168.38ms p(90)=2.81ms p(95)=3.76ms
  iterations..............................................................: 16980565 51455.687821/s
  vus.....................................................................: 2        min=1             max=100
  vus_max.................................................................: 100      min=100           max=100
  
  NETWORK
  data_received...........................................................: 3.4 GB   10 MB/s
  data_sent...............................................................: 4.4 GB   14 MB/s
```

### Key Performance Metrics:

- **Request Throughput**: 51,455 requests per second
- **Response Time**: 95% of responses under 3.14ms
- **Perfect Reliability**: 0 failed requests out of nearly 17 million
- **Cache Hit Rate**: 100% efficiency with 8.4+ million successful cache hits 
- **Scalability**: Maintained performance with up to 100 virtual users