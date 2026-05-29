//go:build noci

/*************************************************************************
 * Copyright 2026 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package testsupport

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultGravwellImage = "gravwell/gravwell:latest"
	// ENV_IMAGE is the environment variable that overrides the Docker image used
	// by LaunchGravwell.
	ENV_IMAGE = "GWCLI_TEST_IMAGE"
)

// LaunchGravwell starts a Docker container running a Gravwell image and waits
// until the /api/ping endpoint responds with HTTP 200. It sets GWCLI_TEST_SERVER
// in the process environment so that testsupport.Server() returns the correct
// address for the duration of the test binary.
//
// The image can be overridden via the GWCLI_TEST_IMAGE environment variable
// (default: "gravwell/gravwell:latest"). If GWCLI_TEST_SERVER is already set
// before calling LaunchGravwell, callers should skip it and use the existing
// instance directly.
//
// Returns the server address string (host:port), a cleanup func that terminates
// the container (always non-nil), and any error. On error, cleanup is a no-op.
func LaunchGravwell() (addr string, cleanup func(), err error) {
	cleanup = func() {} // safe no-op default

	image := defaultGravwellImage
	if v, ok := os.LookupEnv(ENV_IMAGE); ok && v != "" {
		image = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{"80/tcp"},
		WaitingFor: wait.ForHTTP("/api/ping").
			WithPort("80/tcp").
			WithStatusCodeMatcher(func(code int) bool { return code == 200 }).
			WithStartupTimeout(3 * time.Minute),
	}
	ctr, containerErr := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if containerErr != nil {
		return "", func() {}, fmt.Errorf("starting gravwell container: %w", containerErr)
	}

	cleanup = func() {
		_ = ctr.Terminate(context.Background())
	}

	host, hostErr := ctr.Host(ctx)
	if hostErr != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("getting container host: %w", hostErr)
	}
	port, portErr := ctr.MappedPort(ctx, "80")
	if portErr != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("getting container port: %w", portErr)
	}

	addr = host + ":" + port.Port()
	if setErr := os.Setenv(ENV_SERVER, addr); setErr != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("setting %s: %w", ENV_SERVER, setErr)
	}

	return addr, cleanup, nil
}
