package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dedpnd/GophKeeper/internal/logger"
	repository "github.com/dedpnd/GophKeeper/internal/server/adapters/repository/pg"
	"github.com/dedpnd/GophKeeper/internal/server/config"
	"github.com/dedpnd/GophKeeper/internal/server/core"
)

var (
	buildVersion string = "N/A"
	buildDate    string = "N/A"
)

func main() {
	eCfg, err := config.GetConfig()
	if err != nil {
		log.Fatalln(err)
	}

	lg, err := logger.Init("info")
	if err != nil {
		log.Fatalln(err)
	}

	lg.Info(fmt.Sprintf("Build version: %v", buildVersion))
	lg.Info(fmt.Sprintf("Build date: %v", buildDate))

	repo, err := repository.NewDB(context.Background(), lg, eCfg.DSN)
	if err != nil {
		lg.Fatal(err.Error())
	}

	err = core.RunGRPCserver(lg, eCfg.Host, eCfg.JWTkey, repo)
	if err != nil {
		lg.Fatal(err.Error())
	}
}
