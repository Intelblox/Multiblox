module github.com/Intelblox/Multiblox

go 1.22.4

require golang.org/x/sys v0.21.0

require rblxapi v1.0.0

replace rblxapi => ./internal/rblxapi

require regconf v1.0.0

replace regconf => ./internal/regconf

require app v1.0.0

require (
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/shirou/gopsutil/v4 v4.24.5 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
)

require (
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	gopkg.in/toast.v1 v1.0.0-20180812000517-0a84660828b2
)

replace app => ./internal/app
