package core

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/dedpnd/GophKeeper/internal/server/core/domain/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

func NewClient(lg *zap.Logger, addr string, token string, command string) error {
	tlsCredentials, err := loadTLSCredentials()
	if err != nil {
		return fmt.Errorf("cannot load TLS credentials: %s", err)
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(tlsCredentials))
	if err != nil {
		return fmt.Errorf("failed start grpc server: %s", err)
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			lg.With(zap.Error(err)).Error("failed close gRPC client")
		}
	}()

	switch command {
	case "sign-up":
		fmt.Println("-> Create new account")

		ss, err := getUserCredentials()
		if err != nil {
			return fmt.Errorf("failed get user credentials: %s", err)
		}

		client := proto.NewUserClient(conn)
		resp, err := client.Register(context.Background(), &proto.RegiserRequest{
			Login:    ss.login,
			Password: ss.password,
		})

		if err != nil {
			return fmt.Errorf("response finished error: %s", err)
		}

		if resp.Error != "" {
			return fmt.Errorf("response return error: %s", resp.Error)
		}

		fmt.Printf("Token: %s \n", resp.Jwt)
		err = saveAuthToken(resp.Jwt)
		if err != nil {
			return fmt.Errorf("client failed save token: %w", err)
		}
	case "sign-in":
		fmt.Println("-> Sign in with your account")

		ss, err := getUserCredentials()
		if err != nil {
			return fmt.Errorf("failed get user credentials: %s", err)
		}

		client := proto.NewUserClient(conn)
		resp, err := client.Login(context.Background(), &proto.LoginRequest{
			Login:    ss.login,
			Password: ss.password,
		})

		if err != nil {
			return fmt.Errorf("response finished error: %s", err)
		}

		if resp.Error != "" {
			return fmt.Errorf("response return error: %s", resp.Error)
		}

		fmt.Printf("Token: %s \n", resp.Jwt)
		err = saveAuthToken(resp.Jwt)
		if err != nil {
			return fmt.Errorf("client failed save token: %w", err)
		}
	case "read-file":
		fmt.Println("-> Read file")

		md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		client := proto.NewStorageClient(conn)
		resp, err := client.ReadRecord(ctx, &proto.ReadRecordRequest{
			Id: 1,
		})

		if err != nil {
			return fmt.Errorf("response finished error: %s", err)
		}

		if resp.Error != "" {
			return fmt.Errorf("response return error: %s", resp.Error)
		}

		lg.Info("RESPONSE", zap.Int32("ID", resp.Id))
	default:
		fmt.Printf("Command:%s not found! \n", command)
	}

	fmt.Println("Bye!")
	return nil
}

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := os.ReadFile("cert/ca-cert.pem")
	if err != nil {
		return nil, fmt.Errorf("failde load file: %s", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemServerCA) {
		return nil, fmt.Errorf("failed to add server CA's certificate")
	}

	// Create the credentials and return it
	config := &tls.Config{
		RootCAs: certPool,
	}

	return credentials.NewTLS(config), nil
}

/* !UTILS! */

func saveAuthToken(token string) error {
	fmt.Print("Do you want save token in .env? [y/N]: ")

	// Создайте считыватель для ввода из стандартного ввода (консоли)
	reader := bufio.NewReader(os.Stdin)

	// Считайте ответ пользователя
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed read stdin: %w", err)
	}

	// Обрежьте пробелы и символы новой строки из ответа
	response = strings.TrimSpace(response)

	// Проверьте ответ пользователя
	if strings.ToLower(response) == "y" {
		// Откройте файл .env в режиме добавления (append) или создания, если его еще нет
		file, err := os.OpenFile(".env", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open .env file: %w", err)
		}
		defer file.Close()

		// Запишите строку с токеном в формате "JWT=your_token" в файл
		_, err = file.WriteString(fmt.Sprintf("JWT=%s\n", token))
		if err != nil {
			return fmt.Errorf("failed to write token to .env file: %w", err)
		}

		fmt.Println("Token saved in .env file.")
	}

	return nil
}

type userCredentials struct {
	login    string
	password string
}

func getUserCredentials() (userCredentials, error) {
	fmt.Print("Enter your login: ")

	reader := bufio.NewReader(os.Stdin)

	loginResp, err := reader.ReadString('\n')
	if err != nil {
		return userCredentials{}, fmt.Errorf("failed read login stdin: %w", err)
	}

	fmt.Print("Enter your password: ")
	passwordResp, err := reader.ReadString('\n')
	if err != nil {
		return userCredentials{}, fmt.Errorf("failed read password stdin: %w", err)
	}

	return userCredentials{
		login:    strings.TrimSpace(loginResp),
		password: strings.TrimSpace(passwordResp),
	}, nil
}
