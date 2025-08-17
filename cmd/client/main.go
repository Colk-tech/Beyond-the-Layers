package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"time"

	v1 "github.com/Colk-tech/Beyond-the-Layers/gen/v1"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type stats struct {
	avg   time.Duration
	med   time.Duration
	p95   time.Duration
	std   time.Duration
	count int
}

func summarize(ds []time.Duration) stats {
	if len(ds) == 0 {
		return stats{}
	}
	sum := time.Duration(0)
	for _, d := range ds {
		sum += d
	}
	avg := time.Duration(int64(sum) / int64(len(ds)))
	cp := append([]time.Duration(nil), ds...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	med := cp[len(cp)/2]
	p95 := cp[int(math.Ceil(0.95*float64(len(cp))))-1]
	mean := float64(avg)
	vars := 0.0
	for _, d := range ds {
		vars += math.Pow(float64(d)-mean, 2)
	}
	std := time.Duration(math.Sqrt(vars / float64(len(ds))))
	return stats{avg: avg, med: med, p95: p95, std: std, count: len(ds)}
}

func main() {
	defaultAddr := "localhost:50051"
	if env := os.Getenv("ADDR"); env != "" {
		defaultAddr = env
	}
	addr := flag.String("addr", defaultAddr, "server address host:port (e.g., server:50051)")
	path := flag.String("path", "/", "path for Stat")
	follow := flag.Bool("follow", false, "follow symlink")
	timeout := flag.Duration("timeout", 5*time.Second, "RPC timeout")
	warmup := flag.Int("warmup", 5, "number of warmup RPCs (excluded from stats)")
	iters := flag.Int("iters", 50, "number of measured RPCs")
	flag.Parse()

	log.Printf("connecting to %s, path=%s follow=%v warmup=%d iters=%d",
		*addr, *path, *follow, *warmup, *iters)

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

	// ウォームアップ
	for i := 0; i < *warmup; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		_, _ = client.Stat(ctx, &v1.StatRequest{Path: *path, FollowSymlink: *follow})
		cancel()
	}

	// RPC 計測
	rpcDur := make([]time.Duration, 0, *iters)
	var last *v1.StatResponse
	for i := 0; i < *iters; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		start := time.Now()
		res, err := client.Stat(ctx, &v1.StatRequest{
			Path:          *path,
			FollowSymlink: *follow,
		})
		d := time.Since(start)
		cancel()
		if err != nil {
			log.Fatalf("Stat RPC failed at iter=%d: %v", i, err)
		}
		if i == *iters-1 {
			last = res
		}
		rpcDur = append(rpcDur, d)
	}

	// ローカル syscall ベースライン計測（クライアントコンテナ内で同じ path を測る）
	localDur := make([]time.Duration, 0, *iters)
	for i := 0; i < *iters; i++ {
		start := time.Now()
		var st unix.Stat_t
		var err error
		if *follow {
			err = unix.Stat(*path, &st)
		} else {
			err = unix.Lstat(*path, &st)
		}
		d := time.Since(start)
		if err != nil {
			log.Fatalf("local stat failed at iter=%d: %v", i, err)
		}
		localDur = append(localDur, d)
	}

	rpcStats := summarize(rpcDur)
	localStats := summarize(localDur)

	// 結果表示
	if last != nil {
		fmt.Printf("stat result: size=%d mode=%o uid=%d gid=%d type=%s mtime=%d.%09d\n",
			last.Size, last.Mode, last.Uid, last.Gid, last.Type, last.MtimeSec, last.MtimeNsec)
	}
	fmt.Printf("RPC   latency: avg=%s med=%s p95=%s std=%s n=%d\n",
		rpcStats.avg, rpcStats.med, rpcStats.p95, rpcStats.std, rpcStats.count)
	fmt.Printf("Local latency: avg=%s med=%s p95=%s std=%s n=%d\n",
		localStats.avg, localStats.med, localStats.p95, localStats.std, localStats.count)
}
