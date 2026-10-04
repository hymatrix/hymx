package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/urfave/cli/v2"
)

type StopCommandTestSuite struct {
	suite.Suite
}

func (suite *StopCommandTestSuite) TestInvalidPIDKeepsLockFile() {
	oldWd, err := os.Getwd()
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), os.Chdir(suite.T().TempDir()))
	suite.T().Cleanup(func() { require.NoError(suite.T(), os.Chdir(oldWd)) })
	for _, pid := range []string{"0", "-1", "invalid", ""} {
		suite.Run("pid="+pid, func() {
			require.NoError(suite.T(), os.WriteFile(Pid, []byte(pid), 0600))
			app := &cli.App{Commands: cmds}

			err := app.Run([]string{"hymx", "stop", "--checkpoint"})

			assert.ErrorContains(suite.T(), err, "invalid PID")
			data, err := os.ReadFile(Pid)
			require.NoError(suite.T(), err)
			assert.Equal(suite.T(), pid, string(data))
		})
	}
}

func TestStopCommandTestSuite(t *testing.T) {
	suite.Run(t, new(StopCommandTestSuite))
}
