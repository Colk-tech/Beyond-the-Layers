package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	v1 "github.com/Colk-tech/Beyond-the-Layers/gen/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 1) フラグと環境変数から接続先を決定
	defaultAddr := "localhost:50051"
	if env := os.Getenv("ADDR"); env != "" {
		defaultAddr = env
	}
	addr := flag.String("addr", defaultAddr, "server address host:port (e.g., server:50051)")
	path := flag.String("path", "/etc/hosts", "path for Stat")
	follow := flag.Bool("follow", false, "follow symlink")
	timeout := flag.Duration("timeout", 5*time.Second, "RPC timeout")
	flag.Parse()

	log.Printf("connecting to %s", *addr)

	// 2) gRPC 接続 ( Dial は非推奨なので NewClient を使用 )
	conn, err := grpc.NewClient(
		*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Printf("close error: %v", cerr)
		}
	}()

	client := v1.NewSysFSClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	res, err := client.Stat(ctx, &v1.StatRequest{
		Path:          *path,
		FollowSymlink: *follow,
	})
	if err != nil {
		log.Fatalf("Stat RPC failed: %v", err)
	}
	fmt.Printf("size=%d mode=%o uid=%d gid=%d type=%s mtime=%d.%09d\n",
		res.Size, res.Mode, res.Uid, res.Gid, res.Type, res.MtimeSec, res.MtimeNsec)
}
