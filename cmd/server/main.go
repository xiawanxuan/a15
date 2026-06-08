package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"astro-scheduler/internal/api"
	"astro-scheduler/internal/archiver"
	"astro-scheduler/internal/config"
	"astro-scheduler/internal/node"
	"astro-scheduler/internal/notifier"
	"astro-scheduler/internal/scheduler"
	"astro-scheduler/pkg/lock"
	"astro-scheduler/pkg/storage"
	"astro-scheduler/pkg/utils"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := utils.InitLogger(cfg.Log.Level, cfg.Log.File); err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer utils.SyncLogger()

	utils.Sugar.Info("Starting Astro Scheduler Service...")

	distLock, err := lock.NewDistributedLock(cfg.Lock.ToLockConfig())
	if err != nil {
		utils.Sugar.Fatalf("Failed to create distributed lock: %v", err)
	}
	defer func() {
		if distLock != nil {
			_ = distLock.Close()
		}
	}()
	utils.Sugar.Infof("Distributed lock initialized: %s", cfg.Lock.Type)

	objectStorage, err := storage.NewObjectStorage(cfg.Storage.ToStorageConfig())
	if err != nil {
		utils.Sugar.Fatalf("Failed to create object storage: %v", err)
	}
	defer func() {
		if objectStorage != nil {
			_ = objectStorage.Close()
		}
	}()
	utils.Sugar.Infof("Object storage initialized: %s", cfg.Storage.Type)

	alertStore := notifier.NewAlertStore()
	notifierSvc := notifier.NewNotifier(alertStore, cfg.Notification.ToModel())
	notifierSvc.Start()
	defer notifierSvc.Stop()

	nodeStore := node.NewNodeStore()
	nodeManager := node.NewNodeManager(nodeStore, notifierSvc)
	nodeManager.SetHeartbeatTimeout(time.Duration(cfg.Node.HeartbeatTimeout) * time.Second)
	nodeManager.SetCheckInterval(time.Duration(cfg.Node.CheckInterval) * time.Second)
	if cfg.Node.LoadBalancerStrategy != "" {
		nodeManager.SetLoadBalancerStrategy(cfg.Node.LoadBalancerStrategy)
	}
	nodeManager.Start()
	defer nodeManager.Stop()

	dataStore := archiver.NewDataStore()
	archiverSvc := archiver.NewArchiver(
		dataStore,
		notifierSvc,
		objectStorage,
		distLock,
		cfg.Archiver.Bucket,
		cfg.Archiver.BasePath,
	)
	archiverSvc.Start()
	defer archiverSvc.Stop()

	taskStore := scheduler.NewTaskStore()
	schedulerSvc := scheduler.NewScheduler(taskStore, nodeManager, notifierSvc, distLock)
	schedulerSvc.Start()
	defer schedulerSvc.Stop()

	server := api.NewServer(
		schedulerSvc,
		nodeManager,
		archiverSvc,
		notifierSvc,
		cfg.Server.Port,
	)
	if err := server.Start(); err != nil {
		utils.Sugar.Fatalf("Failed to start API server: %v", err)
	}
	defer func() {
		if err := server.Stop(); err != nil {
			utils.Sugar.Errorf("Error stopping server: %v", err)
		}
	}()

	utils.Sugar.Info("Astro Scheduler Service started successfully")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	utils.Sugar.Infof("Received signal %v, shutting down...", sig)
	utils.Sugar.Info("Astro Scheduler Service stopped")
}
