package core

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dedpnd/GophKeeper/internal/server/core/domain/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var defaultPermition fs.FileMode = 0600
var maxMsgSize = 100000648
var errorResponseFinished = "response finished error: %w"
var errorEesponseReturn = "response return error: %w"

func NewClient(lg *zap.Logger, addr string, token string, command string) error {
	tlsCredentials, err := loadTLSCredentials()
	if err != nil {
		return fmt.Errorf("cannot load TLS credentials: %w", err)
	}

	conn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(tlsCredentials),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize), grpc.MaxCallSendMsgSize(maxMsgSize)),
	)
	if err != nil {
		return fmt.Errorf("failed start grpc server: %w", err)
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			lg.With(zap.Error(err)).Error("failed close gRPC client")
		}
	}()

	switch command {
	//nolint:dupl // This legal duplicate
	case "sign-up":
		fmt.Println("-> Create new account")

		ss, err := getUserCredentials()
		if err != nil {
			return fmt.Errorf("failed get user credentials: %w", err)
		}

		client := proto.NewUserClient(conn)
		resp, err := client.Register(context.Background(), &proto.RegiserRequest{
			Login:    ss.login,
			Password: ss.password,
		})

		if err != nil {
			return fmt.Errorf(errorResponseFinished, err)
		}

		if resp.Error != "" {
			return fmt.Errorf(errorEesponseReturn, resp.Error)
		}

		fmt.Printf("Token: %s \n", resp.Jwt)
		err = saveAuthToken(resp.Jwt)
		if err != nil {
			return fmt.Errorf("client failed save token: %w", err)
		}
	//nolint:dupl // This legal duplicate
	case "sign-in":
		fmt.Println("-> Sign in with your account")

		ss, err := getUserCredentials()
		if err != nil {
			return fmt.Errorf("failed get user credentials: %w", err)
		}

		client := proto.NewUserClient(conn)
		resp, err := client.Login(context.Background(), &proto.LoginRequest{
			Login:    ss.login,
			Password: ss.password,
		})

		if err != nil {
			return fmt.Errorf(errorResponseFinished, err)
		}

		if resp.Error != "" {
			return fmt.Errorf(errorEesponseReturn, resp.Error)
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
			return fmt.Errorf(errorResponseFinished, err)
		}
		if respAR.Error != "" {
			return fmt.Errorf(errorEesponseReturn, respAR.Error)
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
			return fmt.Errorf("wrong id file: %w", err)
		}

		respRR, err := client.ReadRecord(ctx, &proto.ReadRecordRequest{
			Id: int32(i),
		})

		if err != nil {
			return fmt.Errorf(errorResponseFinished, err)
		}
		if respRR.Error != "" {
			return fmt.Errorf(errorEesponseReturn, respRR.Error)
		}

		// Type "file"
		if respRR.Type == "file" {
			err = saveFileInDisk(respRR.Name, respRR.Data)
			if err != nil {
				return fmt.Errorf("save file has error: %w", err)
			}
		} else {
			// Type "text"
			fmt.Println(string(respRR.Data))
		}
	case "write-file":
		fmt.Println("-> Write file")

		md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		client := proto.NewStorageClient(conn)

		stream, err := client.WriteRecord(ctx)
		if err != nil {
			return fmt.Errorf(errorResponseFinished, err)
		}

		err = selectWriteData(stream)
		if err != nil {
			return fmt.Errorf("select write data error: %w", err)
		}
	case "delete-file":
		fmt.Println("-> Delete file")

		md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		client := proto.NewStorageClient(conn)
		respAR, err := client.ReadAllRecord(ctx, &proto.ReadAllRecordRequest{})

		if err != nil {
			return fmt.Errorf(errorResponseFinished, err)
		}
		if respAR.Error != "" {
			return fmt.Errorf(errorEesponseReturn, respAR.Error)
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
			return fmt.Errorf("wrong id file: %w", err)
		}

		respDR, err := client.DeleteRecord(ctx, &proto.DeleteRecordRequest{
			Id: int32(i),
		})

		if err != nil {
			return fmt.Errorf(errorResponseFinished, err)
		}

		if respDR.Error != "" {
			return fmt.Errorf(errorEesponseReturn, respDR.Error)
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
		return nil, fmt.Errorf("failde load file: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemServerCA) {
		return nil, fmt.Errorf("failed to add server CA's certificate")
	}

	// Create the credentials and return it
	config := &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	return credentials.NewTLS(config), nil
}

/* !UTILS! */
var errorFailedReadSTDIN = "failed read stdin: %w"

func saveFileInDisk(fileName string, data []byte) error {
	fmt.Println("Where do you want to save the file?")
	fmt.Print("Enter dir path: ")

	// Создайте считыватель для ввода из стандартного ввода (консоли)
	reader := bufio.NewReader(os.Stdin)

	// Считайте ответ пользователя
	r, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf(errorFailedReadSTDIN, err)
	}

	// Обрежьте пробелы и символы новой строки из ответа
	dirPath := strings.TrimSpace(r)
	fullPath := filepath.Join(dirPath, fileName)

	err = os.WriteFile(fullPath, data, defaultPermition)
	if err != nil {
		return fmt.Errorf("failed write data: %w", err)
	}

	fmt.Printf("File save in: %s \n", fullPath)

	return nil
}

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
		return fmt.Errorf(errorFailedReadSTDIN, err)
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
			return fmt.Errorf(errorFailedReadSTDIN, err)
		}

		r = strings.TrimSpace(r)

		i, err := strconv.Atoi(r)
		if err != nil {
			return fmt.Errorf("failed parse int: %w", err)
		}

		fmt.Print("Enter name: ")

		fileName, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf(errorFailedReadSTDIN, err)
		}

		fileName = strings.TrimSpace(fileName)

		switch i {
		case 1:
			fmt.Println("Enter text:")
		//nolint:gomnd // This legal number
		case 2:
			fmt.Println("Enter loggin and password:")
		//nolint:gomnd // This legal number
		case 3:
			fmt.Println("Enter number, name, date and CVV:")
		}

		data, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf(errorFailedReadSTDIN, err)
		}

		data = strings.TrimSpace(data)

		// Отправьте данные по gRPC
		err = stream.Send(&proto.WriteRecordRequest{Name: fileName, Data: []byte(data), Type: "text"})
		if err != nil {
			return fmt.Errorf("stream send has error: %w", err)
		}

		// Закройте поток и получите ответ
		res, err := stream.CloseAndRecv()
		if err != nil {
			return fmt.Errorf("closed stream has error: %w", err)
		}
		if res.Error != "" {
			return fmt.Errorf(errorEesponseReturn, res.Error)
		}
	//nolint:gomnd // This legal number
	case 2:
		fmt.Print("Enter the link to the file: ")

		// Считайте ответ пользователя
		filePath, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf(errorFailedReadSTDIN, err)
		}

		filePath = strings.TrimSpace(filePath)

		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed open file: %w", err)
		}

		fi, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed read stat file: %w", err)
		}

		if fi.Size() > int64(maxMsgSize) {
			return fmt.Errorf("maximum file size should be less: %v bytes", maxMsgSize)
		}

		// Get file name
		baseName := filepath.Base(file.Name())

		// Прочитать файл кусками и отправить
		chunkSize := 4096
		buf := make([]byte, chunkSize)
		for {
			n, err := file.Read(buf)
			if errors.Is(err, io.EOF) {
				// Конец файла, закройте поток
				res, err := stream.CloseAndRecv()
				if err != nil {
					return fmt.Errorf("failed CloseAndRecv: %w", err)
				}
				if res.Error != "" {
					return fmt.Errorf(errorEesponseReturn, res.Error)
				}
				break
			}
			if err != nil {
				return fmt.Errorf("failed read file: %w", err)
			}

			// Отправьте кусок данных
			err = stream.Send(&proto.WriteRecordRequest{Name: baseName, Data: buf[:n], Type: "file"})
			if err != nil {
				return fmt.Errorf("failed send stream: %w", err)
			}
		}

		err = file.Close()
		if err != nil {
			return fmt.Errorf("failed close file: %w", err)
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
		return fmt.Errorf(errorFailedReadSTDIN, err)
	}

	// Обрежьте пробелы и символы новой строки из ответа
	response = strings.TrimSpace(response)

	// Проверьте ответ пользователя
	if strings.ToLower(response) == "y" {
		// Откройте файл .env в режиме добавления (append) или создания, если его еще нет
		file, err := os.OpenFile(".env", os.O_CREATE|os.O_WRONLY, defaultPermition)
		if err != nil {
			return fmt.Errorf("failed to open .env file: %w", err)
		}

		// Запишите строку с токеном в формате "JWT=your_token" в файл
		_, err = file.WriteString(fmt.Sprintf("JWT=%s\n", token))
		if err != nil {
			return fmt.Errorf("failed to write token to .env file: %w", err)
		}

		err = file.Close()
		if err != nil {
			return fmt.Errorf("failed close file: %w", err)
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
