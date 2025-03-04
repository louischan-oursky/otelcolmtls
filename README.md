# What is this?

This repository gives an example of how to set up two otelcol emulating a situation where
a local otelcol sends metrics to another remote otelcol with mTLS.

## How to run this?

1. Assume you are on macOS.
2. You have installed Docker Desktop.
3. (Optional) You are a Nix flake user. If you are, then you can use the prepared flake with direnv.
   Otherwise, you need to install Go and install telemetrygen yourselves. Instructions are not given here. Please figure it out yourselves.

Run the following to start the otelcol and Prometheus.

```sh
docker compose up
```

Run the following to use telemetrygen to generate and send a metric.

```sh
make generate-and-send-metrics
```

Then you visit `http://localhost:9090`, you should see a metric called `gen_total`

## Go example

This repository also contains a minimal Go example of how to configure the Go otel SDK.

You can run it with

```sh
go run .
```

Then you visit `http://localhost:3000`.
