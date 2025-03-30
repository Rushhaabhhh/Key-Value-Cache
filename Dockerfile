FROM rust:1.74-slim as builder

WORKDIR /usr/src/app

RUN cargo new --bin remote_dictionary
WORKDIR /usr/src/app/remote_dictionary

COPY Cargo.toml ./
COPY Cargo.lock* ./

RUN apt-get update && apt-get install -y pkg-config libssl-dev

RUN cargo build --release
RUN rm src/*.rs

COPY src ./src

RUN cargo build --release

FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /usr/src/app/remote_dictionary/target/release/remote_dictionary /app/

EXPOSE 7171

CMD ["/KV_Cache"]
