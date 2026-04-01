package main

import (
	"log"

	appruntime "wincontrol/internal/runtime"
	winservice "wincontrol/internal/windows/service"
)

func main() {
	runner := winservice.Runner{
		Name:    "WinControlActivityService",
		RunFunc: appruntime.ServiceMain,
	}
	if err := runner.Run(); err != nil {
		log.Fatal(err)
	}
}
