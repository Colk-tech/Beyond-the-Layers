package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"

	v1 "github.com/Colk-tech/Beyond-the-Layers/gen/v1"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type server struct {
	v1.UnimplementedSysFSServer
}

// FileType enum への写像
func fileTypeEnum(mode uint32) v1.FileType {
	switch mode & unix.S_IFMT {
	case unix.S_IFREG:
		return v1.FileType_FILE
	case unix.S_IFDIR:
		return v1.FileType_DIR
	case unix.S_IFLNK:
		return v1.FileType_SYMLINK
	default:
		return v1.FileType_OTHER
	}
}

func (s *server) Stat(_ context.Context, req *v1.StatRequest) (*v1.StatResponse, error) {
	if req == nil || req.Path == "" {
		return nil, status.Error(codes.InvalidArgument, "path is required")
	}

	var st unix.Stat_t
	var err error

	if req.FollowSymlink {
		// SymLink を Follow する場合
		err = unix.Stat(req.Path, &st)
	} else {
		// SymLink を Follow しない場合
		err = unix.Lstat(req.Path, &st)
	}

	if err != nil {
		switch {
		case errors.Is(err, unix.ENOENT):
			return nil, status.Error(codes.NotFound, "not found")
		case errors.Is(err, unix.EACCES):
			return nil, status.Error(codes.PermissionDenied, "permission denied")
		default:
			return nil, status.Errorf(codes.Internal, "stat error: %v", err)
		}
	}

	resp := &v1.StatResponse{
		Size:      st.Size,
		Mode:      uint32(st.Mode),
		Uid:       st.Uid,
		Gid:       st.Gid,
		MtimeSec:  st.Mtim.Sec,
		MtimeNsec: st.Mtim.Nsec,
		Type:      fileTypeEnum(uint32(st.Mode)),
	}
	return resp, nil
}

func main() {
	var listen = flag.String("listen", ":50051", "listen address (host:port)")
	flag.Parse()

	gs := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	v1.RegisterSysFSServer(gs, &server{})

	lis, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	log.Printf("sysrpc server listening on %s", *listen)

	if err := gs.Serve(lis); err != nil {
		log.Fatalf("serve error: %v", err)
	}
}
