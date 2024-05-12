//nolint:all // This legal
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/dedpnd/GophKeeper/internal/logger"
	handler "github.com/dedpnd/GophKeeper/internal/server/adapters/handler/grpc"
	interceptors "github.com/dedpnd/GophKeeper/internal/server/adapters/middleware/grpc"
	repository "github.com/dedpnd/GophKeeper/internal/server/adapters/repository/pg"
	"github.com/dedpnd/GophKeeper/internal/server/core/domain/proto"
	"github.com/dedpnd/GophKeeper/internal/server/core/services"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	_ "github.com/lib/pq"
)

var databaseURL string
var listen *bufconn.Listener
var testJWTkey = "12345"
var testCertPath = "../../cert/ca-cert.pem"
var testMasterKey = "1234567812345678"

// var testUser = "test"
// var testUserID = 1

func TestMain(m *testing.M) {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
	}

	err = pool.Client.Ping()
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}

	// pulls an image, creates a container based on it and runs it
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "16.1-alpine3.18",
		Env: []string{
			"POSTGRES_PASSWORD=test",
			"POSTGRES_USER=test",
			"listen_addresses = '*'",
		},
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	hostAndPort := resource.GetHostPort("5432/tcp")
	databaseURL = fmt.Sprintf("postgres://test:test@%s?sslmode=disable", hostAndPort)

	log.Println("Connecting to database on url: ", databaseURL)

	// Tell docker to hard kill the container in 120 seconds
	err = resource.Expire(120)
	if err != nil {
		log.Fatalf("Expire resource has error: %s", err)
	}

	var sqlDB *sql.DB
	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	pool.MaxWait = 20 * time.Second
	if err = pool.Retry(func() error {
		sqlDB, err = sql.Open("postgres", databaseURL)
		if err != nil {
			return fmt.Errorf("Connection has error: %w", err)
		}

		err = sqlDB.Ping()
		if err != nil {
			return fmt.Errorf("Ping has error: %w", err)
		}

		return nil
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	// Run tests
	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}

type clients struct {
	user    proto.UserClient
	storage proto.StorageClient
}

func testServer(ctx context.Context) (clients, func()) {
	buffer := 101024 * 1024
	listen = bufconn.Listen(buffer)

	lg, err := logger.Init("error")
	if err != nil {
		log.Fatalln(err)
	}

	repo, err := repository.NewDB(context.Background(), lg, databaseURL)
	if err != nil {
		lg.Fatal(err.Error())
	}

	baseServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			selector.UnaryServerInterceptor(
				auth.UnaryServerInterceptor(interceptors.GetAuthenticator(testJWTkey)),
				selector.MatchFunc(interceptors.AuthMatcher),
			),
		),
		grpc.ChainStreamInterceptor(
			selector.StreamServerInterceptor(
				auth.StreamServerInterceptor(interceptors.GetAuthenticator(testJWTkey)),
				selector.MatchFunc(interceptors.AuthMatcher),
			),
		),
	)
	userSvc := services.NewUserService(repo)

	// Create user service
	proto.RegisterUserServer(baseServer, &handler.UserHandler{
		Svc:    *userSvc,
		Logger: lg,
		JWTkey: testJWTkey,
	})

	// Create storage service
	storageSvc := services.NewStorageService(repo)
	proto.RegisterStorageServer(baseServer, &handler.StorageHandler{
		Svc:       *storageSvc,
		Logger:    lg,
		MasterKey: testMasterKey,
	})

	go func() {
		if err := baseServer.Serve(listen); err != nil {
			log.Printf("error serving server: %v", err)
		}
	}()

	conn, err := grpc.DialContext(ctx, "",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			//nolint:wrapcheck // This legal return
			return listen.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	closer := func() {
		err := listen.Close()
		if err != nil {
			log.Printf("error closing listener: %v", err)
		}
		baseServer.Stop()
	}

	uClient := proto.NewUserClient(conn)
	sClient := proto.NewStorageClient(conn)

	return clients{
		user:    uClient,
		storage: sClient,
	}, closer
}
