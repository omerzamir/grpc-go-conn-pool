package grpc

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/meateam/grpc-go-conn-pool/grpc/types"
	"google.golang.org/grpc"
)

// ConnPool is a pool of grpc.ClientConns.
type ConnPool = types.ConnPool

var _ ConnPool = &roundRobinConnPool{}
var _ ConnPool = &singleConnPool{}

// singleConnPool is a special case for a single connection.
type singleConnPool struct {
	*grpc.ClientConn
}

func (p *singleConnPool) Conn() *grpc.ClientConn { return p.ClientConn }
func (p *singleConnPool) Num() int               { return 1 }

type roundRobinConnPool struct {
	conns []*grpc.ClientConn

	idx uint32 // access via sync/atomic
}

func (p *roundRobinConnPool) Num() int {
	return len(p.conns)
}

func (p *roundRobinConnPool) Conn() *grpc.ClientConn {
	i := atomic.AddUint32(&p.idx, 1)
	return p.conns[i%uint32(len(p.conns))]
}

func (p *roundRobinConnPool) Close() error {
	var errs multiError
	for _, conn := range p.conns {
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (p *roundRobinConnPool) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	return p.Conn().Invoke(ctx, method, args, reply, opts...)
}

func (p *roundRobinConnPool) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return p.Conn().NewStream(ctx, desc, method, opts...)
}

// multiError represents errors from multiple conns in the group.
type multiError []error

func (m multiError) Error() string {
	s, n := "", 0
	for _, e := range m {
		if e != nil {
			if n == 0 {
				s = e.Error()
			}
			n++
		}
	}
	switch n {
	case 0:
		return "(0 errors)"
	case 1:
		return s
	case 2:
		return s + " (and 1 other error)"
	}
	return fmt.Sprintf("%s (and %d other errors)", s, n-1)
}
