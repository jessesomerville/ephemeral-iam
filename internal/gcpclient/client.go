/*
Copyright © 2021 Jesse Somerville

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package gcpclient

import (
	"context"
	"fmt"
	"sync"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"google.golang.org/api/option"
)

var gcpClient *credentials.IamCredentialsClient
var once sync.Once

// GetGCPClient gets a gcloud client using the local gcloud configuration
func GetGCPClient() *credentials.IamCredentialsClient {
	once.Do(func() {
		var err error
		gcpClient, err = newGcpClient()
		handleErr(err)
	})
	return gcpClient
}

// WithReason creates a client SDK with the provided reason field
func WithReason(reason string) (*credentials.IamCredentialsClient, error) {
	ctx := context.Background()
	gcpClientWithReason, err := credentials.NewIamCredentialsClient(ctx, option.WithRequestReason(reason))
	if err != nil {
		return nil, fmt.Errorf("Failed to create a client SDK with a reason field: %v", err)
	}
	return gcpClientWithReason, nil
}

func newGcpClient() (*credentials.IamCredentialsClient, error) {
	ctx := context.Background()
	gcpClient, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a client SDK: %v", err)
	}
	return gcpClient, nil
}
