package main

import (
    goclog "github.com/byx-darwin/go-tools/go-common/log"

    "github.com/x/demo/internal/base/conf"
    "github.com/x/demo/internal/base/server"
)

func main() {
    if err := conf.Init(); err != nil {
        goclog.L().Fatal("load config", "error", err)
    }

    cfg := conf.Get()
    if err := goclog.Init(goclog.Config{
        Level:  cfg.Log.Level,
        Format: cfg.Log.Format,
        Mode:   cfg.Log.Mode,
    }, goclog.ReleaseInfo{
        ServiceName: cfg.Server.Registry.Name,
        Environment: cfg.Env,
    }); err != nil {
        goclog.L().Fatal("init log", "error", err)
    }
    defer goclog.Close()

    server.Run()
}
