module github.com/travis-g/dice-golem

go 1.18

require (
	github.com/armon/go-metrics v0.4.1
	github.com/bwmarrin/discordgo v0.27.2-0.20240315152229-33ee38cbf271
	github.com/dustin/go-humanize v1.0.1
	github.com/gocarina/gocsv v0.0.0-20231116093920-b87c2d0e983a
	github.com/lithammer/fuzzysearch v1.1.8
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/sethvargo/go-envconfig v1.0.1
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/travis-g/dice v0.0.0-20230511165330-b68d50b20159
	go.uber.org/zap v1.27.0
	golang.org/x/text v0.14.0
	gopkg.in/redis.v3 v3.6.4
)

replace (
	github.com/bwmarrin/discordgo => ../discordgo
)

require golang.org/x/net v0.22.0 // indirect

require (
	github.com/Knetic/govaluate v3.0.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/garyburd/redigo v1.6.4 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/mitchellh/mapstructure v1.5.0
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.20.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	gopkg.in/bsm/ratelimit.v1 v1.0.0-20170922094635-f56db5e73a5e // indirect
)
