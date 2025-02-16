package main

import (
	"bufio"
	"context"
	"fmt"
	pb "gRPC/proto"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)


func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)

	// Get username
	fmt.Print("Enter username: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	username := scanner.Text()

	_, err = client.Login(context.Background(), &pb.LoginRequest{UserName: username})
	if err != nil {
		log.Fatalf("Login failed: %v", err)
	}

	// Start chat stream
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.ChatStream(ctx)
	if err != nil {
		log.Fatalf("Failed to open stream: %v", err)
	}

	//send inital authentication message
	stream.Send(&pb.Messages{From: username})

	//receive msgs
	go func(){
		for{
			msg, err := stream.Recv()
			if err != nil {
				log.Fatalf("connection closed: %v", err)
			}

			switch msg.Type {
			case pb.MessageType_MESSAGE:
				fmt.Printf("\n[%s] %s > %s\n> ", msg.Timestamp, msg.From, msg.Content)
			case pb.MessageType_USERJOIN:
				fmt.Printf("\n[System] %s joined the chat\n> ", msg.From)
			case pb.MessageType_USERLEFT:
				fmt.Printf("\n[System] %s left the chat\n> ", msg.From)
			}
		}
		
	}()

	//send msgs
	go func() {
		for {
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("send msgs")
			for scanner.Scan() {
				text := scanner.Text()
				if text == "" {
					continue
				}

				msg := &pb.Messages{
					From: username,
					Content: text,
					Timestamp: time.Now().Format(time.DateTime),
					Type: pb.MessageType_MESSAGE,
					To: "all",
				}

				if err := stream.Send(msg); err != nil {
					log.Printf("Failed to send message: %v", err)
					return
				}
				fmt.Print("> ")
			}	
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nDisconnecting...")
}