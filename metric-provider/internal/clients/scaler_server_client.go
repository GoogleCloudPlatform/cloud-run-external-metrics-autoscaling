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
	"fmt"

	pb "crema/metric-provider/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ScalerServerClient is a wrapper around the Scaler gRPC client.
type ScalerServerClient struct {
	conn   *grpc.ClientConn
	client pb.ScalerClient
}

// ScalerServer creates a new ScalerServer client.
func ScalerServer(address string) (*ScalerServerClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("did not connect: %w", err)
	}
	return &ScalerServerClient{
		conn:   conn,
		client: pb.NewScalerClient(conn),
	}, nil
}

// Close closes the connection to the Scaler service.
func (c *ScalerServerClient) Close() error {
	return c.conn.Close()
}

// Scale sends a scale request to the Scaler service.
func (c *ScalerServerClient) Scale(ctx context.Context, req *pb.ScaleRequest) (*pb.ScaleResponse, error) {
	return c.client.Scale(ctx, req)
}
