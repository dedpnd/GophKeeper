package core

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/dedpnd/GophKeeper/internal/server/core/domain/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var maxMsgSize = 100000648

func NewClient(lg *zap.Logger, addr string, token string, command string) error {
	tlsCredentials, err := loadTLSCredentials()
	if err != nil {
		return fmt.Errorf("cannot load TLS credentials: %s", err)
	}

	conn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(tlsCredentials),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize), grpc.MaxCallSendMsgSize(maxMsgSize)),
	)
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
		respAR, err := client.ReadAllRecord(ctx, &proto.ReadAllRecordRequest{})

		if err != nil {
			return fmt.Errorf("response finished error: %s", err)
		}
		if respAR.Error != "" {
			return fmt.Errorf("response return error: %s", respAR.Error)
		}

		if len(respAR.Units) == 0 {
			fmt.Println("Not found files. Bye!")
			return nil
		}

		fmt.Println("Available files:")
		for _, v := range respAR.Units {
			// TODO: Откуда 0 ? Size slice ?
			if v.Id > 0 {
				fmt.Printf("[%v] - %s \n", v.Id, v.Name)
			}
		}

		i, err := selectReadFile()
		if err != nil {
			return fmt.Errorf("wrong id file: %s", err)
		}

		respRR, err := client.ReadRecord(ctx, &proto.ReadRecordRequest{
			Id: int32(i),
		})

		if err != nil {
			return fmt.Errorf("response finished error: %s", err)
		}
		if respRR.Error != "" {
			return fmt.Errorf("response return error: %s", respRR.Error)
		}

		fmt.Println(string(respRR.Data))
	case "write-file":
		fmt.Println("-> Write file")

		md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		client := proto.NewStorageClient(conn)

		stream, err := client.WriteRecord(ctx)
		if err != nil {
			return fmt.Errorf("response finished error: %s", err)
		}

		err = selectWriteData(stream)
		if err != nil {
			return fmt.Errorf("select write data error: %s", err)
		}
	case "delete-file":
		fmt.Println("-> Delete file")

		md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		client := proto.NewStorageClient(conn)
		respAR, err := client.ReadAllRecord(ctx, &proto.ReadAllRecordRequest{})

		if err != nil {
			return fmt.Errorf("response finished error: %s", err)
		}
		if respAR.Error != "" {
			return fmt.Errorf("response return error: %s", respAR.Error)
		}

		if len(respAR.Units) == 0 {
			fmt.Println("Not found files. Bye!")
			return nil
		}

		fmt.Println("Available files:")
		for _, v := range respAR.Units {
			// TODO: Откуда 0 ? Size slice ?
			if v.Id > 0 {
				fmt.Printf("[%v] - %s \n", v.Id, v.Name)
			}
		}

		i, err := selectReadFile()
		if err != nil {
			return fmt.Errorf("wrong id file: %s", err)
		}

		respDR, err := client.DeleteRecord(ctx, &proto.DeleteRecordRequest{
			Id: int32(i),
		})

		if err != nil {
			return fmt.Errorf("response finished error: %s", err)
		}

		if respDR.Error != "" {
			return fmt.Errorf("response return error: %s", respDR.Error)
		}

		fmt.Println("File delete!")
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

func selectWriteData(stream proto.Storage_WriteRecordClient) error {
	fmt.Println("What you want send on server?")
	fmt.Println("[1] - Text")
	fmt.Println("[2] - File")
	fmt.Print("Enter a number: ")

	// Создайте считыватель для ввода из стандартного ввода (консоли)
	reader := bufio.NewReader(os.Stdin)

	// Считайте ответ пользователя
	r, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed read stdin: %w", err)
	}

	// Обрежьте пробелы и символы новой строки из ответа
	r = strings.TrimSpace(r)

	i, err := strconv.Atoi(r)
	if err != nil {
		return fmt.Errorf("failed parse int: %w", err)
	}

	switch i {
	case 1:
		// TODO: Выбрать тип и сделать сохранение в зависимости от него
		fmt.Println("What do you want to save?")
		fmt.Println("[1] - Custom text")
		fmt.Println("[2] - Login | Password")
		fmt.Println("[3] - Credit card")
		fmt.Print("Enter a number: ")

		r, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed read stdin: %w", err)
		}

		r = strings.TrimSpace(r)

		i, err := strconv.Atoi(r)
		if err != nil {
			return fmt.Errorf("failed parse int: %w", err)
		}

		fmt.Print("Enter name: ")

		fileName, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed read stdin: %w", err)
		}

		fileName = strings.TrimSpace(fileName)

		switch i {
		case 1:
			fmt.Println("Enter text:")
		case 2:
			fmt.Println("Enter loggin and password:")
		case 3:
			fmt.Println("Enter number, name, date and CVV:")
		}

		data, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed read stdin: %w", err)
		}

		data = strings.TrimSpace(data)

		// Отправьте данные по gRPC
		err = stream.Send(&proto.WriteRecordRequest{Name: fileName, Data: []byte(data), Type: "text"})
		if err != nil {
			return fmt.Errorf("stream send has error: %s", err)
		}

		// Закройте поток и получите ответ
		res, err := stream.CloseAndRecv()
		if err != nil {
			return fmt.Errorf("closed stream has error: %s", err)
		}
		if res.Error != "" {
			return fmt.Errorf("response return error: %s", res.Error)
		}
	case 2:
		fmt.Print("Enter the link to the file: ")

		// Считайте ответ пользователя
		filePath, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed read stdin: %w", err)
		}

		filePath = strings.TrimSpace(filePath)

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed open file: %w", err)
		}
		defer file.Close()

		fi, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed read stat file: %w", err)
		}

		if fi.Size() > int64(maxMsgSize) {
			return fmt.Errorf("maximum file size should be less: %v bytes", maxMsgSize)
		}

		// Прочитать файл кусками и отправить
		buf := make([]byte, 4096)
		for {
			n, err := file.Read(buf)
			if err == io.EOF {
				// Конец файла, закройте поток
				res, err := stream.CloseAndRecv()
				if err != nil {
					return fmt.Errorf("failed CloseAndRecv: %w", err)
				}
				if res.Error != "" {
					return fmt.Errorf("response return error: %s", res.Error)
				}
				break
			}
			if err != nil {
				return fmt.Errorf("failed read file: %w", err)
			}

			// Отправьте кусок данных
			err = stream.Send(&proto.WriteRecordRequest{Name: file.Name(), Data: buf[:n], Type: "file"})
			if err != nil {
				return fmt.Errorf("failed send stream: %w", err)
			}
		}
	}

	fmt.Println("File write!")

	return nil
}

func selectReadFile() (int, error) {
	fmt.Print("Select ID file: ")

	// Создайте считыватель для ввода из стандартного ввода (консоли)
	reader := bufio.NewReader(os.Stdin)

	// Считайте ответ пользователя
	response, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("failed read stdin: %w", err)
	}

	// Обрежьте пробелы и символы новой строки из ответа
	response = strings.TrimSpace(response)

	i, err := strconv.Atoi(response)
	if err != nil {
		return 0, fmt.Errorf("failed parse int: %w", err)
	}

	return i, nil
}

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
