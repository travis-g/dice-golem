module github.com/travis-g/dice-golem

go 1.24

require (
	github.com/armon/go-metrics v0.4.1
	github.com/bwmarrin/discordgo v0.29.0
	github.com/dustin/go-humanize v1.0.1
	github.com/gocarina/gocsv v0.0.0-20240520201108-78e41c74b4b1
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/lithammer/fuzzysearch v1.1.8
	github.com/redis/go-redis/v9 v9.10.0
	github.com/sethvargo/go-envconfig v1.3.0
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/travis-g/dice v0.0.0-20240426015834-4e95258df453
	go.uber.org/zap v1.27.0
	golang.org/x/text v0.26.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	golang.org/x/net v0.22.0 // indirect
)

require (
	github.com/Knetic/govaluate v3.0.0+incompatible // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/mitchellh/mapstructure v1.5.0
	github.com/pkg/errors v0.9.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
)

replace github.com/armon/go-metics => github.com/hashicorp/go-metrics v0.5.4
