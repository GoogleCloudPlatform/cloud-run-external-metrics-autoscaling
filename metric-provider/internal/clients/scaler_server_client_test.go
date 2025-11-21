// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clients

import (
	"context"
	"net"
	"testing"

	pb "crema/metric-provider/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type mockScalerServer struct {
	pb.UnimplementedScalerServer
}

func (s *mockScalerServer) Scale(ctx context.Context, in *pb.ScaleRequest) (*pb.ScaleResponse, error) {
	return &pb.ScaleResponse{}, nil
}

func TestClient(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	pb.RegisterScalerServer(s, &mockScalerServer{})
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Serve(lis)
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := &ScalerServerClient{
		conn:   conn,
		client: pb.NewScalerClient(conn),
	}

	t.Run("Scale", func(t *testing.T) {
		req := &pb.ScaleRequest{}
		_, err := client.Scale(context.Background(), req)
		if err != nil {
			t.Errorf("Scale() error = %v, wantErr %v", err, false)
		}
	})

	s.Stop()
	if err := <-errCh; err != nil {
		t.Fatalf("Server exited with error: %v", err)
	}
}
