package main

import (
	"fmt"
	"log"

	"github.com/dedpnd/GophKeeper/internal/agent/config"
	"github.com/dedpnd/GophKeeper/internal/agent/core"
	"github.com/dedpnd/GophKeeper/internal/logger"
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

	fmt.Println("*************************************")
	fmt.Println("Welcome GophKepeer client")
	fmt.Printf("Build version: %v \n", buildVersion)
	fmt.Printf("Build date: %v \n", buildDate)
	fmt.Println("*************************************")

	if eCfg.Command == "" {
		fmt.Println("Support command -c:")
		fmt.Println("sign-up - create new account")
		fmt.Println("sign-in - sign in with your account")
	}

	err = core.NewClient(lg, eCfg.ServerAddr, eCfg.JWT, eCfg.Command)
	if err != nil {
		lg.Sugar().Fatalf("grpc client: %s", err.Error())
	}
}
