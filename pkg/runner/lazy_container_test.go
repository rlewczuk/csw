package runner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubContainerRunner struct {
	runCalls int
	closed   bool
}

func (s *stubContainerRunner) RunCommand(command string) (string, int, error) {
	s.runCalls++
	return "ok:" + command, 0, nil
}

func (s *stubContainerRunner) RunCommandWithOptions(command string, options CommandOptions) (string, int, error) {
	s.runCalls++
	_ = options
	return "ok:" + command, 0, nil
}

func (s *stubContainerRunner) RunCommandWithOptionsDetailed(command string, options CommandOptions) (string, string, int, error) {
	s.runCalls++
	_ = options
	return "ok:" + command, "", 0, nil
}

func (s *stubContainerRunner) Close() error {
	s.closed = true
	return nil
}

func (s *stubContainerRunner) Identity() ContainerIdentity {
	return ContainerIdentity{UID: 1000, GID: 1000, UserName: "tester", GroupName: "tester", HomeDir: "/home/tester"}
}

func (s *stubContainerRunner) ImageInfo() ContainerImageInfo {
	return ContainerImageInfo{Name: "busybox", Tag: "latest", Version: "latest"}
}

func TestNewLazyContainerRunnerDefersContainerCreationUntilRun(t *testing.T) {
	originalFactory := NewContainerRunnerFactoryForTest()
	t.Cleanup(func() {
		SetNewContainerRunnerFactoryForTest(originalFactory)
	})

	factoryCalls := 0
	stub := &stubContainerRunner{}
	SetNewContainerRunnerFactoryForTest(func(config ContainerConfig) (ContainerRunner, error) {
		factoryCalls++
		_ = config
		return stub, nil
	})

	runnerInstance, err := NewLazyContainerRunner(ContainerConfig{ImageName: "busybox:latest"})
	require.NoError(t, err)
	require.NotNil(t, runnerInstance)

	assert.Equal(t, 0, factoryCalls)
	assert.Equal(t, "busybox", runnerInstance.ImageInfo().Name)
	assert.Equal(t, 0, factoryCalls)

	output, exitCode, runErr := runnerInstance.RunCommand("echo hi")
	require.NoError(t, runErr)
	assert.Equal(t, "ok:echo hi", output)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, 1, factoryCalls)
	assert.Equal(t, 1, stub.runCalls)

	_, _, runErr = runnerInstance.RunCommand("echo second")
	require.NoError(t, runErr)
	assert.Equal(t, 1, factoryCalls)
	assert.Equal(t, 2, stub.runCalls)
}

func TestLazyContainerRunnerCloseWithoutStart(t *testing.T) {
	runnerInstance, err := NewLazyContainerRunner(ContainerConfig{ImageName: "busybox:latest"})
	require.NoError(t, err)

	require.NoError(t, runnerInstance.Close())
	_, _, runErr := runnerInstance.RunCommand("echo hi")
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "container is closed")
}

func TestLazyContainerRunnerPropagatesFactoryError(t *testing.T) {
	originalFactory := NewContainerRunnerFactoryForTest()
	t.Cleanup(func() {
		SetNewContainerRunnerFactoryForTest(originalFactory)
	})

	SetNewContainerRunnerFactoryForTest(func(config ContainerConfig) (ContainerRunner, error) {
		_ = config
		return nil, fmt.Errorf("boom")
	})

	runnerInstance, err := NewLazyContainerRunner(ContainerConfig{ImageName: "busybox:latest"})
	require.NoError(t, err)

	_, _, runErr := runnerInstance.RunCommand("echo hi")
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "failed to start container runner")
}
