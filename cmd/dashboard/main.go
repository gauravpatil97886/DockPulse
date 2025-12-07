package main

import (
	"fmt"
	"log"

	"devops-dashboard/internal/docker"
	"devops-dashboard/internal/ui/dashboard"
)

func main() {
	fmt.Println("Starting DevOps Dashboard...")

	// Check Docker
	err := docker.CheckDockerConnection()
	if err != nil {
		log.Fatalf("Docker error: %v", err)
	}

	// Start UI
	app, err := dashboard.NewDashboardUI()
	if err != nil {
		log.Fatalf("UI error: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
