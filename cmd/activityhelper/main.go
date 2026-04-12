package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	appruntime "windowsuseruptimecontrol/internal/runtime"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := appruntime.HelperMain(ctx); err != nil {
		log.Fatal(err)
	}
}
