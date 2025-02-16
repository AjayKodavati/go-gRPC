package main

import (
	"context"
	pb "gRPC/proto"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type user struct {
	username string
	stream   chan *pb.Messages
	online   bool
}

type Server struct {
	pb.UnimplementedChatServiceServer
	Users sync.Map
}

func (s *Server) Login(ctx context.Context, request *pb.LoginRequest) (*pb.LoginResponse, error) {
	userName := request.UserName
	if userName == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid username")
	}
	return &pb.LoginResponse{Token: "dummy"}, nil
}

func (s *Server) ChatStream(stream pb.ChatService_ChatStreamServer) error {
	msg, err := stream.Recv()
	if err != nil {
		return status.Error(codes.Aborted, "failed to receive data from user");
	}

	userName := msg.From
	if userName == "" {
		return status.Error(codes.Unavailable, "username not available")
	}

	user := &user{
		username: msg.From,
		stream:   make(chan *pb.Messages, 100),
		online:   true,
	}
	s.Users.Store(msg.From, user)

	s.broadcast(&pb.Messages{
		Type:  pb.MessageType_USERJOIN,
		From: userName,
		Timestamp: time.Now().Format(time.DateTime),
	})

	defer func() {
		user.online = false
		s.Users.Delete(userName)
		s.broadcast(&pb.Messages{
			Type: pb.MessageType_USERLEFT,
			From: userName,
			Timestamp: time.Now().Format(time.DateTime),
		})
	}()
	
	//send msgs 
	go func() {
		for msg := range user.stream {
			if err := stream.Send(msg); err != nil {
				log.Printf("Error sending to %s: %v", userName, err)
				return
			}
		}
	}()

	// Receive message from the client
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		msg.Timestamp = time.Now().Format(time.DateTime)
		msg.Type =pb.MessageType_MESSAGE

		if msg.To == "all" {
			s.broadcast(msg)
		} else {
			s.sendprivate(msg)
		}
	}
}

func (s *Server) broadcast(msg *pb.Messages) {
	s.Users.Range(func(key, value any) bool {
		user := value.(*user)
		if user.online {
			select {
			case user.stream <- msg:
			default:
				log.Printf("Couldn't send to %s (buffer full)", user.username)
			}
		}
		return true
	})
}

func (s *Server) sendprivate(msg *pb.Messages) {
	if val, ok := s.Users.Load(msg.To); ok {
		recv := val.(*user)
		if recv.online {
			recv.stream <- msg
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterChatServiceServer(s, &Server{})

	log.Println("Server running on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

