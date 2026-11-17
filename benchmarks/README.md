# Benchmark reports

`v0.2.0-darwin-arm64.json` records a reproducible comparison against the
`v0.1.7` tag using the fixed 42-image `testdata` corpus, ONNX Runtime 1.29.0,
two warm repeats, and stable JSON comparison with elapsed fields removed.

The timing values are host-specific. Recreate the accuracy comparison with
`scripts/api_reference.py --baseline v0.1.7` and
`scripts/python_reference.py --baseline v0.1.7`; use
`scripts/benchmark_native.py` for the Python SDK benchmark. Go batch timing in
the report uses an in-process Engine harness with `MaxInFlight` set to 1, 2,
and 4. The report also records the existing Go/Python text differences so
future releases can detect cross-end drift separately from within-end JSON
compatibility.
