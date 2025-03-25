FROM rust:latest

WORKDIR /app

COPY . .

RUN cargo build --release

EXPOSE 7171

CMD ["./target/release/key_value_cache"]
