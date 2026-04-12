package main

import (
	"log"

	appruntime "windowsuseruptimecontrol/internal/runtime"
	winservice "windowsuseruptimecontrol/internal/windows/service"
)

func main() {
	runner := winservice.Runner{
		Name:    "WindowsUserUptimeControlActivityService",
		RunFunc: appruntime.ServiceMain,
	}
	if err := runner.Run(); err != nil {
		log.Fatal(err)
	}
}
