package examples

import _ "github.com/NefixEstrada/agen"

// Fully supported:
//
//go:generate go run github.com/NefixEstrada/agen/cmd/agen -v --clean --config config/with_unimplemented.yml --target ex_streetlights_kafka       ../_testdata/examples/streetlights-kafka.yaml
//go:generate go run github.com/NefixEstrada/agen/cmd/agen -v --clean --target ex_converted_streetlights ../_testdata/examples/converted-streetlights.yaml
//go:generate go run github.com/NefixEstrada/agen/cmd/agen -v --clean --target ex_converted_params        ../_testdata/examples/converted-params.yaml

// Runnable Redis demo (package redisdemo consumed by redis/main.go):
//
//go:generate go run github.com/NefixEstrada/agen/cmd/agen -v --clean --package-name redisdemo --target ex_redis/api ../_testdata/examples/redis-demo.yaml
