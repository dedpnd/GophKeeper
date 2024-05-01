// Package core contains the basic logic of the application.
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

var maxMsgSize = 100000648
var defaultPermition fs.FileMode = 0600
var errorResponseFinished = "response finished error: %w"
var errorEesponseReturn = "response return error: %w"

// NewClient initializes gRPC client.
func NewClient(lg *zap.Logger, addr string, token string, command string) error {
	// Get TLS cert
	tlsCredentials, err := loadTLSCredentials()
	if err != nil {
		return fmt.Errorf("cannot load TLS credentials: %w", err)
	}

	// Connect to gRPC server
	conn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(tlsCredentials),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize), grpc.MaxCallSendMsgSize(maxMsgSize)),
	)
	if err != nil {
		return fmt.Errorf("failed start grpc server: %w", err)
	}

	// Close gRPC connection
	defer func() {
		err := conn.Close()
		if err != nil {
			lg.With(zap.Error(err)).Error("failed close gRPC client")
		}
	}()

	// Depending on the command, we choose the logic of behavior
	switch command {
	//nolint:dupl // This legal duplicate
	case "sign-up":
		fmt.Println("-> Create new account")

		// Get user credentials from stdin tui
		ss, err := getUserCredentials()
		if err != nil {
			return fmt.Errorf("failed get user credentials: %w", err)
		}

		// Create client
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

		// Do you want to save the token?
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

		// Set authorization in gRPC metadata
		md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		// Create client
		client := proto.NewStorageClient(conn)
		respAR, err := client.ReadAllRecord(ctx, &proto.ReadAllRecordRequest{})

		if err != nil {
			return fmt.Errorf(errorResponseFinished, err)
		}
		if respAR.Error != "" {
			return fmt.Errorf(errorEesponseReturn, respAR.Error)
		}

		// If there are no files, exit
		if len(respAR.Units) == 0 {
			fmt.Println("Not found files. Bye!")
			return nil
		}

		// Showing the available files
		fmt.Println("Available files:")
		for _, v := range respAR.Units {
			// TODO: Откуда 0 ? Size slice ?
			if v.Id > 0 {
				fmt.Printf("[%v] - %s \n", v.Id, v.Name)
			}
		}

		// Selecting a file to download
		i, err := selectReadFile()
		if err != nil {
			return fmt.Errorf("wrong id file: %w", err)
		}

		// Request to read the file
		respRR, err := client.ReadRecord(ctx, &proto.ReadRecordRequest{
			Id: int32(i),
		})

		if err != nil {
			return fmt.Errorf(errorResponseFinished, err)
		}
		if respRR.Error != "" {
			return fmt.Errorf(errorEesponseReturn, respRR.Error)
		}

		// If the file type is file
		if respRR.Type == "file" {
			err = saveFileInDisk(respRR.Name, respRR.Data)
			if err != nil {
				return fmt.Errorf("save file has error: %w", err)
			}
		} else {
			// Else type is text
			fmt.Println(string(respRR.Data))
		}
	case "write-file":
		fmt.Println("-> Write file")

		// Set authorization in gRPC metadata
		md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		// Create client
		client := proto.NewStorageClient(conn)
		stream, err := client.WriteRecord(ctx)
		if err != nil {
			return fmt.Errorf(errorResponseFinished, err)
		}

		// Selecting the file type and the file we want to save
		err = selectWriteData(stream)
		if err != nil {
			return fmt.Errorf("select write data error: %w", err)
		}
	case "delete-file":
		fmt.Println("-> Delete file")

		// Set authorization in gRPC metadata
		md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		// Create client
		client := proto.NewStorageClient(conn)
		respAR, err := client.ReadAllRecord(ctx, &proto.ReadAllRecordRequest{})

		if err != nil {
			return fmt.Errorf(errorResponseFinished, err)
		}
		if respAR.Error != "" {
			return fmt.Errorf(errorEesponseReturn, respAR.Error)
		}

		// If there are no files, exit
		if len(respAR.Units) == 0 {
			fmt.Println("Not found files. Bye!")
			return nil
		}

		// Showing the available files
		fmt.Println("Available files:")
		for _, v := range respAR.Units {
			// TODO: Откуда 0 ? Size slice ?
			if v.Id > 0 {
				fmt.Printf("[%v] - %s \n", v.Id, v.Name)
			}
		}

		// Select a file to delete
		i, err := selectReadFile()
		if err != nil {
			return fmt.Errorf("wrong id file: %w", err)
		}

		// Request for deletion
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

// loadTLSCredentials loading certificates.
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

// saveFileInDisk saving files to disk.
func saveFileInDisk(fileName string, data []byte) error {
	fmt.Println("Where do you want to save the file?")
	fmt.Print("Enter dir path: ")

	// Create a reader for input from standard input (console)
	reader := bufio.NewReader(os.Stdin)

	// Consider the user's response
	r, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf(errorFailedReadSTDIN, err)
	}

	// Trim the spaces and newline characters from the response
	dirPath := strings.TrimSpace(r)
	fullPath := filepath.Join(dirPath, fileName)

	err = os.WriteFile(fullPath, data, defaultPermition)
	if err != nil {
		return fmt.Errorf("failed write data: %w", err)
	}

	fmt.Printf("File save in: %s \n", fullPath)

	return nil
}

// selectWriteData selecting a file to download.
func selectWriteData(stream proto.Storage_WriteRecordClient) error {
	fmt.Println("What you want send on server?")
	fmt.Println("[1] - Text")
	fmt.Println("[2] - File")
	fmt.Print("Enter a number: ")

	// Create a reader for input from standard input (console)
	reader := bufio.NewReader(os.Stdin)

	// Consider the user's response
	r, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf(errorFailedReadSTDIN, err)
	}

	// Trim the spaces and newline characters from the response
	r = strings.TrimSpace(r)

	i, err := strconv.Atoi(r)
	if err != nil {
		return fmt.Errorf("failed parse int: %w", err)
	}

	switch i {
	case 1:
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

		// Send the gRPC data
		err = stream.Send(&proto.WriteRecordRequest{Name: fileName, Data: []byte(data), Type: "text"})
		if err != nil {
			return fmt.Errorf("stream send has error: %w", err)
		}

		// Close the stream and get a response
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

		// Consider the user's response
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

		// Read the file in chunks and send
		chunkSize := 4096
		buf := make([]byte, chunkSize)
		for {
			n, err := file.Read(buf)
			if errors.Is(err, io.EOF) {
				// End of file, close the stream
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

			// Send a piece of data
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

// selectReadFile select a file to read.
func selectReadFile() (int, error) {
	fmt.Print("Select ID file: ")

	// Create a reader for input from standard input (console)
	reader := bufio.NewReader(os.Stdin)

	// Consider the user's response
	response, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("failed read stdin: %w", err)
	}

	// Trim the spaces and newline characters from the response
	response = strings.TrimSpace(response)

	// Converting the answer to a digit
	i, err := strconv.Atoi(response)
	if err != nil {
		return 0, fmt.Errorf("failed parse int: %w", err)
	}

	return i, nil
}

// saveAuthToken saving the token to the .env file.
func saveAuthToken(token string) error {
	fmt.Print("Do you want save token in .env? [y/N]: ")

	// Create a reader for input from standard input (console)
	reader := bufio.NewReader(os.Stdin)

	// Consider the user's response
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf(errorFailedReadSTDIN, err)
	}

	// Trim the spaces and newline characters from the response
	response = strings.TrimSpace(response)

	// Check the user's response
	if strings.ToLower(response) == "y" {
		// Open the file .env in append or create mode, if it doesn't exist yet
		file, err := os.OpenFile(".env", os.O_CREATE|os.O_WRONLY, defaultPermition)
		if err != nil {
			return fmt.Errorf("failed to open .env file: %w", err)
		}

		// Write the string with the token in the format "JWT=your_token" to the file
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

// getUserCredentials get a pair of username and password from the user.
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
