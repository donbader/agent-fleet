module github.com/donbader/agent-fleet

go 1.24.3

require (
	github.com/donbader/agent-fleet/gateway v0.0.0
	github.com/spf13/cobra v1.9.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
)

replace github.com/donbader/agent-fleet/gateway => ./images/gateway
