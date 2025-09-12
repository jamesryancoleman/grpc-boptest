# grpc-boptest

a driver for interacting with a boptest testcase via `devctrl`

The `key` of a request is expected to be of the format:
`boptest://{test_case_id}/{point_name}`.

For example:
`boptest://bestest_air/{point_name}`.
