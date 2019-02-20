package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hand-writing-authentication-team/credential-store/events"
	"github.com/hand-writing-authentication-team/credential-store/queue"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

type config struct {
	mqHost     string
	mqPort     string
	mqUsername string
	mqPassword string

	QC *queue.Queue
	ch <-chan amqp.Delivery
}

var goroutineDelta = make(chan int)
var serverConfig config

func start() {
	serverConfig.mqHost = os.Getenv("MQ_HOST")
	serverConfig.mqPort = os.Getenv("MQ_PORT")
	serverConfig.mqUsername = os.Getenv("MQ_USER")
	serverConfig.mqPassword = os.Getenv("MQ_PASSWORD")

	if strings.TrimSpace(serverConfig.mqHost) == "" || strings.TrimSpace(serverConfig.mqPassword) == "" || strings.TrimSpace(serverConfig.mqPort) == "" || strings.TrimSpace(serverConfig.mqUsername) == "" {
		log.Fatal("one of the mq config env is not set!")
		os.Exit(1)
	}

	queueClient, err := queue.NewQueueInstance(serverConfig.mqHost, serverConfig.mqPort, serverConfig.mqUsername, serverConfig.mqPassword)
	if err != nil {
		os.Exit(1)
	}
	serverConfig.QC = queueClient
	return
}

func main() {

	log.Info("start to bootstrap credential-store server")
	start()
	gentlyExit()
	// set up listener logic
	var err error
	serverConfig.ch, err = serverConfig.QC.Consume("credstoreIn")
	if err != nil {
		log.Fatal("queue is not declared")
		os.Exit(1)
	}

	go forever()

	numGoroutines := 0
	for diff := range goroutineDelta {
		numGoroutines += diff
		if numGoroutines == 0 {
			os.Exit(0)
		}
	}
}

// Conceptual code
func forever() {
	log.Info("server started.")
	foreverRunner := make(chan bool)
	go func() {
		for d := range serverConfig.ch {
			err := events.GenericEventHandler(d.Body, serverConfig.QC)
			if err != nil {
				log.Infof("met a error that is %s", err)
			}
		}
	}()
	log.Info("waiting...")
	<-foreverRunner
}

func gentlyExit() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		err := serverConfig.QC.DestroyQueueInstance()
		if err != nil {
			log.WithField("error", err).Fatal("queue connection close failed")
			os.Exit(1)
		} else {
			log.Info("queue connection closed properly, gently quitting")
			os.Exit(0)
		}
	}()
}

func printer(msg string) {
	goroutineDelta <- -1
	log.Infof("%s", msg)
}
